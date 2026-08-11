package systemd

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/go-units"
	godbus "github.com/godbus/dbus/v5"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const usecPerSecond = 1_000_000

func parseCPUQuota(cpu string) (uint64, error) {
	v, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value %q: %w", cpu, err)
	}
	if v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) || v > float64(math.MaxUint64)/usecPerSecond {
		return 0, fmt.Errorf("CPU value must be positive and finite, got %q", cpu)
	}
	quota := uint64(math.Round(v * usecPerSecond))
	if quota == 0 {
		return 0, fmt.Errorf("CPU value %q is too small", cpu)
	}
	return quota, nil
}

type deviceLimit struct {
	Path  string
	Value uint64
}

func bpsDevices(devices []sablier.BlkioThrottleDevice) ([]deviceLimit, error) {
	out := make([]deviceLimit, 0, len(devices))
	for _, d := range devices {
		rate, err := units.RAMInBytes(d.Rate)
		if err != nil {
			return nil, fmt.Errorf("invalid rate %q for device %s: %w", d.Rate, d.Path, err)
		}
		if rate < 0 {
			return nil, fmt.Errorf("rate for device %s must be non-negative, got %q", d.Path, d.Rate)
		}
		out = append(out, deviceLimit{Path: d.Path, Value: uint64(rate)})
	}
	return out, nil
}

func iopsDevices(devices []sablier.BlkioThrottleDevice) ([]deviceLimit, error) {
	out := make([]deviceLimit, 0, len(devices))
	for _, d := range devices {
		iops, err := strconv.ParseUint(d.Rate, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid iops value %q for device %s: %w", d.Rate, d.Path, err)
		}
		out = append(out, deviceLimit{Path: d.Path, Value: iops})
	}
	return out, nil
}

// buildProperties converts a resource profile into systemd unit properties.
// Fields omitted from profile are reset when they were set in clear.
func buildProperties(profile sablier.ResourceProfile, clear sablier.ResourceProfile) ([]dbus.Property, error) {
	var props []dbus.Property

	if profile.CPU != "" {
		quota, err := parseCPUQuota(profile.CPU)
		if err != nil {
			return nil, err
		}
		props = append(props, dbus.Property{Name: "CPUQuotaPerSecUSec", Value: godbus.MakeVariant(quota)})
	} else if clear.CPU != "" {
		props = append(props, dbus.Property{Name: "CPUQuotaPerSecUSec", Value: godbus.MakeVariant(uint64(math.MaxUint64))})
	}

	if profile.Memory != "" {
		mem, err := units.RAMInBytes(profile.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory value %q: %w", profile.Memory, err)
		}
		if mem <= 0 {
			return nil, fmt.Errorf("memory value must be positive, got %q", profile.Memory)
		}
		props = append(props, dbus.Property{Name: "MemoryMax", Value: godbus.MakeVariant(uint64(mem))})
	} else if clear.Memory != "" {
		props = append(props, dbus.Property{Name: "MemoryMax", Value: godbus.MakeVariant(uint64(math.MaxUint64))})
	}

	if profile.BlkioWeight != 0 {
		props = append(props, dbus.Property{Name: "IOWeight", Value: godbus.MakeVariant(uint64(profile.BlkioWeight))})
	} else if clear.BlkioWeight != 0 {
		props = append(props, dbus.Property{Name: "IOWeight", Value: godbus.MakeVariant(uint64(math.MaxUint64))})
	}

	if len(profile.BlkioWeightDevice) > 0 {
		devices := make([]deviceLimit, 0, len(profile.BlkioWeightDevice))
		for _, d := range profile.BlkioWeightDevice {
			devices = append(devices, deviceLimit{Path: d.Path, Value: uint64(d.Weight)})
		}
		props = append(props, dbus.Property{Name: "IODeviceWeight", Value: godbus.MakeVariant(devices)})
	}

	if len(profile.BlkioDeviceReadBps) > 0 {
		devices, err := bpsDevices(profile.BlkioDeviceReadBps)
		if err != nil {
			return nil, err
		}
		props = append(props, dbus.Property{Name: "IOReadBandwidthMax", Value: godbus.MakeVariant(devices)})
	}

	if len(profile.BlkioDeviceWriteBps) > 0 {
		devices, err := bpsDevices(profile.BlkioDeviceWriteBps)
		if err != nil {
			return nil, err
		}
		props = append(props, dbus.Property{Name: "IOWriteBandwidthMax", Value: godbus.MakeVariant(devices)})
	}

	if len(profile.BlkioDeviceReadIOps) > 0 {
		devices, err := iopsDevices(profile.BlkioDeviceReadIOps)
		if err != nil {
			return nil, err
		}
		props = append(props, dbus.Property{Name: "IOReadIOPSMax", Value: godbus.MakeVariant(devices)})
	}

	if len(profile.BlkioDeviceWriteIOps) > 0 {
		devices, err := iopsDevices(profile.BlkioDeviceWriteIOps)
		if err != nil {
			return nil, err
		}
		props = append(props, dbus.Property{Name: "IOWriteIOPSMax", Value: godbus.MakeVariant(devices)})
	}

	return props, nil
}

// applyResources applies a resource profile to a unit at runtime. Runtime
// properties are dropped on daemon-reload or unit restart.
func (p *Provider) applyResources(ctx context.Context, name string, profile, clear sablier.ResourceProfile) error {
	props, err := buildProperties(profile, clear)
	if err != nil {
		return err
	}

	resets := make([]dbus.Property, 0, len(props))
	resetNames := make(map[string]struct{})
	for _, prop := range props {
		if isDeviceProperty(prop.Name) {
			resetNames[prop.Name] = struct{}{}
		}
	}
	for _, name := range deviceProperties(clear) {
		resetNames[name] = struct{}{}
	}
	for _, name := range []string{"IODeviceWeight", "IOReadBandwidthMax", "IOWriteBandwidthMax", "IOReadIOPSMax", "IOWriteIOPSMax"} {
		if _, ok := resetNames[name]; !ok {
			continue
		}
		resets = append(resets, dbus.Property{
			Name:  name,
			Value: godbus.MakeVariant([]deviceLimit{}),
		})
	}
	if len(resets) == 0 && len(props) == 0 {
		return nil
	}
	return p.Con.SetUnitPropertiesContext(ctx, name, true, append(resets, props...)...)
}

func deviceProperties(profile sablier.ResourceProfile) []string {
	var names []string
	if len(profile.BlkioWeightDevice) > 0 {
		names = append(names, "IODeviceWeight")
	}
	if len(profile.BlkioDeviceReadBps) > 0 {
		names = append(names, "IOReadBandwidthMax")
	}
	if len(profile.BlkioDeviceWriteBps) > 0 {
		names = append(names, "IOWriteBandwidthMax")
	}
	if len(profile.BlkioDeviceReadIOps) > 0 {
		names = append(names, "IOReadIOPSMax")
	}
	if len(profile.BlkioDeviceWriteIOps) > 0 {
		names = append(names, "IOWriteIOPSMax")
	}
	return names
}

func (p *Provider) scaleConfig(ctx context.Context, name string) (sablier.ScaleConfig, error) {
	labels, err := p.readManagedLabels(ctx, name)
	if err != nil {
		return sablier.ScaleConfig{}, err
	}
	return sablier.ScaleConfigFromLabels(labels), nil
}

func isDeviceProperty(name string) bool {
	switch name {
	case "IODeviceWeight", "IOReadBandwidthMax", "IOWriteBandwidthMax", "IOReadIOPSMax", "IOWriteIOPSMax":
		return true
	default:
		return false
	}
}
