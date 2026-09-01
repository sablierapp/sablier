package systemd

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestSystemdProvider_InstanceStart(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
	}})
	p := newProviderForTest(t, m, time.Second, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	assert.NilError(t, p.InstanceStart(ctx, "web.service"))
	assert.DeepEqual(t, m.Started(), []string{"web.service"})
	assert.Equal(t, len(m.Stopped()), 0)
}

func TestSystemdProvider_InstanceStart_ActiveResources(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nActiveCPU=0.5\n"),
	}})
	p := newProviderForTest(t, m, time.Second, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	assert.NilError(t, p.InstanceStart(ctx, "web.service"))
	assert.DeepEqual(t, m.Started(), []string{"web.service"})

	props := m.SetProps("web.service")
	assert.Equal(t, len(props), 1)
	assert.Equal(t, props[0].Name, "CPUQuotaPerSecUSec")
	assert.Equal(t, props[0].Value.Value(), uint64(500_000))
}

func TestSystemdProvider_InstanceStart_InvalidScaleConfigDoesNotStart(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{
		{name: "web.service", fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nActiveCPU=invalid\n")},
	})
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.InstanceStart(t.Context(), "web.service")
	assert.ErrorContains(t, err, "invalid CPU value")
	assert.Equal(t, len(m.Started()), 0)
}

func TestSystemdProvider_InstanceStart_ResourceErrorDoesNotStart(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nActiveCPU=0.5\n"),
	}})
	m.SetUnitPropertiesError(errors.New("cannot apply properties"))
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.InstanceStart(t.Context(), "web.service")
	assert.ErrorContains(t, err, "cannot apply active resource profile")
	assert.Equal(t, len(m.Started()), 0)
}

func TestSystemdProvider_InstanceStart_RejectsUnmanagedUnit(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service"}})
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.InstanceStart(t.Context(), "web.service")
	assert.ErrorContains(t, err, "not enabled for Sablier")
	assert.Equal(t, len(m.Started()), 0)
}

func TestSystemdProvider_InstanceStart_ClearsMissingActiveResources(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nIdleReplicas=1\nIdleCPU=0.25\nIdleMemory=32m\nIdleBlkioWeight=100\nIdleBlkioDeviceReadBps=/dev/sda:1m\n"),
	}})
	p := newProviderForTest(t, m, time.Second, nil)

	assert.NilError(t, p.InstanceStart(t.Context(), "web.service"))
	props := m.SetProps("web.service")
	assert.Equal(t, len(props), 4)
	assert.Equal(t, props[0].Name, "IOReadBandwidthMax")
	assert.Equal(t, props[0].Value.Signature().String(), "a(st)")
	assert.Equal(t, props[1].Name, "CPUQuotaPerSecUSec")
	assert.Equal(t, props[1].Value.Value(), uint64(math.MaxUint64))
	assert.Equal(t, props[2].Name, "MemoryMax")
	assert.Equal(t, props[2].Value.Value(), uint64(math.MaxUint64))
	assert.Equal(t, props[3].Name, "IOWeight")
	assert.Equal(t, props[3].Value.Value(), uint64(math.MaxUint64))
}

func TestSystemdProvider_InstanceStart_LoadsUnloadedUnit(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
	}})
	m.UnloadUnit("web.service")
	p := newProviderForTest(t, m, time.Second, nil)

	assert.NilError(t, p.InstanceStart(t.Context(), "web.service"))
	assert.DeepEqual(t, m.Started(), []string{"web.service"})
}
