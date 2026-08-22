package systemd

import (
	"math"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestParseCPUQuota(t *testing.T) {
	tests := []struct {
		name    string
		cpu     string
		want    uint64
		wantErr bool
	}{
		{name: "quarter core", cpu: "0.25", want: 250_000},
		{name: "half a core", cpu: "0.5", want: 500_000},
		{name: "one core", cpu: "1", want: 1_000_000},
		{name: "two cores", cpu: "2", want: 2_000_000},
		{name: "zero", cpu: "0", wantErr: true},
		{name: "invalid", cpu: "abc", wantErr: true},
		{name: "negative", cpu: "-1", wantErr: true},
		{name: "infinite", cpu: "Inf", wantErr: true},
		{name: "not a number", cpu: "NaN", wantErr: true},
		{name: "too small", cpu: "0.0000001", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCPUQuota(tt.cpu)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestBuildProperties(t *testing.T) {
	profile := sablier.ResourceProfile{
		CPU:         "0.5",
		Memory:      "128m",
		BlkioWeight: 100,
		BlkioWeightDevice: []sablier.BlkioWeightDevice{
			{Path: "/dev/sda", Weight: 500},
		},
		BlkioDeviceReadBps: []sablier.BlkioThrottleDevice{
			{Path: "/dev/sda", Rate: "10m"},
		},
		BlkioDeviceWriteBps: []sablier.BlkioThrottleDevice{
			{Path: "/dev/sda", Rate: "5m"},
		},
		BlkioDeviceReadIOps: []sablier.BlkioThrottleDevice{
			{Path: "/dev/sda", Rate: "100"},
		},
		BlkioDeviceWriteIOps: []sablier.BlkioThrottleDevice{
			{Path: "/dev/sda", Rate: "50"},
		},
	}

	got, err := buildProperties(profile, sablier.ResourceProfile{})
	assert.NilError(t, err)

	props := make(map[string]any, len(got))
	for _, p := range got {
		props[p.Name] = p.Value.Value()
	}
	assert.DeepEqual(t, props, map[string]any{
		"CPUQuotaPerSecUSec":  uint64(500_000),
		"MemoryMax":           uint64(134217728),
		"IOWeight":            uint64(100),
		"IODeviceWeight":      []deviceLimit{{Path: "/dev/sda", Value: 500}},
		"IOReadBandwidthMax":  []deviceLimit{{Path: "/dev/sda", Value: 10485760}},
		"IOWriteBandwidthMax": []deviceLimit{{Path: "/dev/sda", Value: 5242880}},
		"IOReadIOPSMax":       []deviceLimit{{Path: "/dev/sda", Value: 100}},
		"IOWriteIOPSMax":      []deviceLimit{{Path: "/dev/sda", Value: 50}},
	})
}

func TestBuildProperties_Empty(t *testing.T) {
	got, err := buildProperties(sablier.ResourceProfile{}, sablier.ResourceProfile{})
	assert.NilError(t, err)
	assert.Equal(t, len(got), 0)
}

func TestBuildProperties_ClearMissingLimits(t *testing.T) {
	got, err := buildProperties(sablier.ResourceProfile{}, sablier.ResourceProfile{CPU: "0.25", Memory: "32m"})
	assert.NilError(t, err)
	assert.Equal(t, len(got), 2)
	assert.Equal(t, got[0].Name, "CPUQuotaPerSecUSec")
	assert.Equal(t, got[0].Value.Value(), uint64(math.MaxUint64))
	assert.Equal(t, got[1].Name, "MemoryMax")
	assert.Equal(t, got[1].Value.Value(), uint64(math.MaxUint64))
}

func TestBuildProperties_InvalidMemory(t *testing.T) {
	_, err := buildProperties(sablier.ResourceProfile{Memory: "bogus"}, sablier.ResourceProfile{})
	assert.Assert(t, err != nil)

	_, err = buildProperties(sablier.ResourceProfile{Memory: "-1m"}, sablier.ResourceProfile{})
	assert.Assert(t, err != nil)

	_, err = buildProperties(sablier.ResourceProfile{Memory: "0"}, sablier.ResourceProfile{})
	assert.Assert(t, err != nil)
}

func TestSystemdProvider_ApplyResources_ResetsDeviceProperties(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service"}})
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.applyResources(t.Context(), "web.service", sablier.ResourceProfile{
		CPU: "0.5",
		BlkioWeightDevice: []sablier.BlkioWeightDevice{
			{Path: "/dev/sda", Weight: 100},
		},
		BlkioDeviceReadBps: []sablier.BlkioThrottleDevice{
			{Path: "/dev/sdb", Rate: "10m"},
		},
	}, sablier.ResourceProfile{})
	assert.NilError(t, err)

	got := m.SetProps("web.service")
	assert.Equal(t, len(got), 5)
	assert.Equal(t, got[0].Name, "IODeviceWeight")
	assert.Equal(t, got[0].Value.Signature().String(), "a(st)")
	assert.Equal(t, len(got[0].Value.Value().([][]any)), 0)
	assert.Equal(t, got[1].Name, "IOReadBandwidthMax")
	assert.Equal(t, got[1].Value.Signature().String(), "a(st)")
	assert.Equal(t, len(got[1].Value.Value().([][]any)), 0)
	assert.Equal(t, got[2].Name, "CPUQuotaPerSecUSec")
	assert.Equal(t, got[2].Value.Value(), uint64(500_000))
	assert.Equal(t, got[3].Name, "IODeviceWeight")
	assert.Equal(t, got[3].Value.Signature().String(), "a(st)")
	assert.DeepEqual(t, got[3].Value.Value(), [][]any{{"/dev/sda", uint64(100)}})
	assert.Equal(t, got[4].Name, "IOReadBandwidthMax")
	assert.Equal(t, got[4].Value.Signature().String(), "a(st)")
	assert.DeepEqual(t, got[4].Value.Value(), [][]any{{"/dev/sdb", uint64(10 * 1024 * 1024)}})
}

func TestSystemdProvider_ApplyResources_ClearsDeviceOnlyProfile(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service"}})
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.applyResources(t.Context(), "web.service", sablier.ResourceProfile{}, sablier.ResourceProfile{
		BlkioDeviceReadBps: []sablier.BlkioThrottleDevice{{Path: "/dev/sda", Rate: "1m"}},
	})
	assert.NilError(t, err)

	props := m.SetProps("web.service")
	assert.Equal(t, len(props), 1)
	assert.Equal(t, props[0].Name, "IOReadBandwidthMax")
	assert.Equal(t, props[0].Value.Signature().String(), "a(st)")
	assert.Equal(t, len(props[0].Value.Value().([][]any)), 0)
}
