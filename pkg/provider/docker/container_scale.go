package docker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/blkiodev"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// minBlkioDeviceAPIVersion is the earliest Docker daemon API version that honors
// per-device blkio limits (BlkioWeightDevice, BlkioDevice{Read,Write}{Bps,IOps})
// on a running container via `docker update`. Older daemons return 200 OK but
// silently drop these fields.
// See moby/moby#52650.
const minBlkioDeviceAPIVersion = "1.55"

// parseCPUNano converts a decimal CPU value (e.g. "0.5", "2") to Docker nanocores.
// 1 CPU = 1,000,000,000 nanocores.
func parseCPUNano(cpu string) (int64, error) {
	v, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value %q: %w", cpu, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("CPU value must be non-negative, got %q", cpu)
	}
	return int64(v * 1e9), nil
}

// parseMemoryBytes converts a human-readable memory string (e.g. "128m", "1g")
// to bytes using Docker-style suffixes (b, k, m, g).
func parseMemoryBytes(memory string) (int64, error) {
	b, err := units.RAMInBytes(memory)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value %q: %w", memory, err)
	}
	return b, nil
}

// parseBpsRate converts a human-readable throughput string (e.g. "10m", "100k")
// to bytes per second using Docker-style suffixes.
func parseBpsRate(s string) (uint64, error) {
	v, err := units.RAMInBytes(s)
	if err != nil {
		return 0, fmt.Errorf("invalid bps rate %q: %w", s, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("bps rate must be non-negative, got %q", s)
	}
	return uint64(v), nil
}

// parseIOpsRate converts a plain integer string (e.g. "100") to an IOPS count.
func parseIOpsRate(s string) (uint64, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid iops value %q: %w", s, err)
	}
	return v, nil
}

// applyResources updates the resource limits of a running container using cgroup
// constraints (docker update). Fields with zero/empty values are left unchanged.
func (p *Provider) applyResources(ctx context.Context, name string, profile sablier.ResourceProfile) error {
	// Per-device blkio limits are silently dropped by daemons older than
	// API 1.55 (moby/moby#52650): the update succeeds but the cgroup is never
	// changed. Warn so the misconfiguration is visible rather than failing quietly.
	if profile.HasBlkioDeviceLimits() && p.apiVersion != "" &&
		versions.LessThan(p.apiVersion, minBlkioDeviceAPIVersion) {
		p.l.WarnContext(ctx, "per-device blkio throttling requires a newer Docker daemon; these limits will be silently ignored",
			"container", name,
			"daemon_api_version", p.apiVersion,
			"required_api_version", minBlkioDeviceAPIVersion,
			"reference", "https://github.com/moby/moby/issues/52650",
		)
	}

	resources := &container.Resources{}

	if profile.CPU != "" {
		v, err := parseCPUNano(profile.CPU)
		if err != nil {
			return err
		}
		resources.NanoCPUs = v
	}

	if profile.Memory != "" {
		v, err := parseMemoryBytes(profile.Memory)
		if err != nil {
			return err
		}
		resources.Memory = v
		// Docker requires MemorySwap >= Memory in the same update call.
		// Setting MemorySwap equal to Memory satisfies the constraint and
		// disables swap for the container.
		resources.MemorySwap = v
	}

	if profile.BlkioWeight != 0 {
		resources.BlkioWeight = profile.BlkioWeight
	}

	for _, d := range profile.BlkioWeightDevice {
		resources.BlkioWeightDevice = append(resources.BlkioWeightDevice,
			&blkiodev.WeightDevice{Path: d.Path, Weight: d.Weight})
	}

	for _, d := range profile.BlkioDeviceReadBps {
		rate, err := parseBpsRate(d.Rate)
		if err != nil {
			p.l.WarnContext(ctx, "invalid blkio-device-read-bps rate, skipping",
				"device", d.Path, "rate", d.Rate, "error", err)
			continue
		}
		resources.BlkioDeviceReadBps = append(resources.BlkioDeviceReadBps,
			&blkiodev.ThrottleDevice{Path: d.Path, Rate: rate})
	}

	for _, d := range profile.BlkioDeviceWriteBps {
		rate, err := parseBpsRate(d.Rate)
		if err != nil {
			p.l.WarnContext(ctx, "invalid blkio-device-write-bps rate, skipping",
				"device", d.Path, "rate", d.Rate, "error", err)
			continue
		}
		resources.BlkioDeviceWriteBps = append(resources.BlkioDeviceWriteBps,
			&blkiodev.ThrottleDevice{Path: d.Path, Rate: rate})
	}

	for _, d := range profile.BlkioDeviceReadIOps {
		rate, err := parseIOpsRate(d.Rate)
		if err != nil {
			p.l.WarnContext(ctx, "invalid blkio-device-read-iops value, skipping",
				"device", d.Path, "rate", d.Rate, "error", err)
			continue
		}
		resources.BlkioDeviceReadIOps = append(resources.BlkioDeviceReadIOps,
			&blkiodev.ThrottleDevice{Path: d.Path, Rate: rate})
	}

	for _, d := range profile.BlkioDeviceWriteIOps {
		rate, err := parseIOpsRate(d.Rate)
		if err != nil {
			p.l.WarnContext(ctx, "invalid blkio-device-write-iops value, skipping",
				"device", d.Path, "rate", d.Rate, "error", err)
			continue
		}
		resources.BlkioDeviceWriteIOps = append(resources.BlkioDeviceWriteIOps,
			&blkiodev.ThrottleDevice{Path: d.Path, Rate: rate})
	}

	result, err := p.Client.ContainerUpdate(ctx, name, client.ContainerUpdateOptions{
		Resources: resources,
	})
	if err != nil {
		return fmt.Errorf("cannot update resources for container %s: %w", name, err)
	}
	if len(result.Warnings) > 0 {
		p.l.WarnContext(ctx, "warnings from container resource update", "name", name, "warnings", result.Warnings)
	}
	return nil
}
