package systemd

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestSystemdProvider_InstanceStop(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
	}})
	p := newProviderForTest(t, m, time.Second, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	assert.NilError(t, p.InstanceStop(ctx, "web.service"))
	assert.DeepEqual(t, m.Stopped(), []string{"web.service"})
}

func TestSystemdProvider_InstanceStop_ScaleModeKeepsRunning(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nIdleReplicas=1\nIdleCPU=0.25\n"),
	}})
	p := newProviderForTest(t, m, time.Second, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	assert.NilError(t, p.InstanceStop(ctx, "web.service"))
	assert.Equal(t, len(m.Stopped()), 0)

	props := m.SetProps("web.service")
	assert.Equal(t, len(props), 1)
	assert.Equal(t, props[0].Name, "CPUQuotaPerSecUSec")
	assert.Equal(t, props[0].Value.Value(), uint64(250_000))
}

func TestSystemdProvider_InstanceStop_ScaleConfigErrorDoesNotStop(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	m.SetGetPropertiesError(errors.New("dbus unavailable"))
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.InstanceStop(t.Context(), "web.service")
	assert.ErrorContains(t, err, "cannot read scale config")
	assert.Equal(t, len(m.Stopped()), 0)
}

func TestSystemdProvider_InstanceStop_ResourceErrorDoesNotStop(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nIdleReplicas=1\nIdleCPU=0.25\n"),
	}})
	m.SetUnitPropertiesError(errors.New("cannot apply properties"))
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.InstanceStop(t.Context(), "web.service")
	assert.ErrorContains(t, err, "cannot apply idle resources")
	assert.Equal(t, len(m.Stopped()), 0)
}

func TestSystemdProvider_InstanceStop_ReplicasCappedWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nIdleReplicas=2\n"),
	}})
	p := newProviderForTest(t, m, time.Second, logger)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	assert.NilError(t, p.InstanceStop(ctx, "web.service"))
	assert.Equal(t, len(m.Stopped()), 0)
	assert.Assert(t, strings.Contains(buf.String(), "capped at 1"), "logs: %s", buf.String())
}

func TestSystemdProvider_InstanceStop_RejectsUnmanagedUnit(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, time.Second, nil)

	err := p.InstanceStop(t.Context(), "web.service")
	assert.ErrorContains(t, err, "not enabled for Sablier")
	assert.Equal(t, len(m.Stopped()), 0)
}

func TestSystemdProvider_InstanceStop_ClearsMissingIdleResources(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nIdleReplicas=1\nActiveCPU=0.5\nActiveMemory=64m\nActiveBlkioWeight=500\nActiveBlkioDeviceReadBps=/dev/sda:2m\n"),
	}})
	p := newProviderForTest(t, m, time.Second, nil)

	assert.NilError(t, p.InstanceStop(t.Context(), "web.service"))
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
