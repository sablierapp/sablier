package systemd_test

import (
	"context"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

const testUnit = "sablier-test.service"

func TestSystemdProvider_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if shared == nil {
		t.Skip("set SABLIER_SYSTEMD_INTEGRATION=1 to run systemd integration tests")
	}

	p := setupSystemd(t)
	ctx := t.Context()

	t.Run("InstanceList", func(t *testing.T) {
		instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
		assert.NilError(t, err)

		// The test unit should appear in the list.
		found := false
		for _, inst := range instances {
			if inst.Name == testUnit {
				found = true
				assert.DeepEqual(t, inst.Groups, []string{"test"})
				break
			}
		}
		assert.Assert(t, found, "expected unit %q in instance list", testUnit)
	})

	t.Run("InstanceGroups", func(t *testing.T) {
		groups, err := p.InstanceGroups(ctx)
		assert.NilError(t, err)

		names, ok := groups["test"]
		assert.Assert(t, ok, "expected group 'test' to exist")

		found := slices.Contains(names, testUnit)
		assert.Assert(t, found, "expected unit %q in group 'test'", testUnit)
	})

	t.Run("StartAndInspect", func(t *testing.T) {
		// Unit should be inactive initially (freshly created).
		info, err := p.InstanceInspect(ctx, testUnit)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)

		// Start the unit.
		err = p.InstanceStart(ctx, testUnit)
		assert.NilError(t, err)

		// Poll InstanceInspect until ready (systemd job settles + unit runs).
		var ready bool
		for i := range 30 {
			info, err = p.InstanceInspect(ctx, testUnit)
			assert.NilError(t, err)

			if info.Status == sablier.InstanceStatusReady {
				ready = true
				break
			}
			t.Logf("inspect attempt %d: status=%s", i+1, info.Status)
			time.Sleep(time.Second)
		}
		assert.Assert(t, ready, "expected unit to become ready, last status: %s", info.Status)
		assert.Equal(t, info.Name, testUnit)
	})

	t.Run("Stop", func(t *testing.T) {
		err := p.InstanceStop(ctx, testUnit)
		assert.NilError(t, err)

		info, err := p.InstanceInspect(ctx, testUnit)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
	})

	t.Run("InstanceEvents", func(t *testing.T) {
		// Start the unit first.
		err := p.InstanceStart(ctx, testUnit)
		assert.NilError(t, err)

		// Wait until it's running.
		ready := false
		for range 30 {
			info, err := p.InstanceInspect(ctx, testUnit)
			assert.NilError(t, err)
			if info.Status == sablier.InstanceStatusReady {
				ready = true
				break
			}
			time.Sleep(time.Second)
		}
		assert.Assert(t, ready, "expected unit to become ready before subscribing")

		// Start the event stream with a cancelable context.
		eventsCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		stream := p.InstanceEvents(eventsCtx, provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventStopped},
		})

		// Stop the unit externally (simulate external stop).
		err = p.InstanceStop(ctx, testUnit)
		assert.NilError(t, err)

		// Wait for the notification.
		select {
		case info, ok := <-stream.Events:
			assert.Assert(t, ok, "events channel closed unexpectedly")
			assert.Equal(t, info.Type, provider.InstanceEventStopped)
			assert.Equal(t, info.Info.Name, testUnit)
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for stop notification")
		}
	})

	t.Run("ResourceProperties", func(t *testing.T) {
		const resourceUnit = "sablier-resource-test.service"
		shared.CreateUnit(t, resourceUnit, "Enable=true\nIdleReplicas=1\nIdleCPU=0.25\nIdleMemory=32m\nIdleBlkioDeviceReadBps=/dev/null:1m\nActiveCPU=0.5\nActiveMemory=64m\nActiveBlkioDeviceReadBps=/dev/null:2m")

		assert.NilError(t, p.InstanceStart(ctx, resourceUnit))
		props, err := p.UnitTypePropertiesForTest(ctx, resourceUnit, "Service")
		assert.NilError(t, err)
		assert.Equal(t, props["CPUQuotaPerSecUSec"], uint64(500_000))
		assert.Equal(t, props["MemoryMax"], uint64(64*1024*1024))
		assert.DeepEqual(t, props["IOReadBandwidthMax"], [][]any{{"/dev/null", uint64(2 * 1024 * 1024)}})

		assert.NilError(t, p.InstanceStop(ctx, resourceUnit))
		props, err = p.UnitTypePropertiesForTest(ctx, resourceUnit, "Service")
		assert.NilError(t, err)
		assert.Equal(t, props["CPUQuotaPerSecUSec"], uint64(250_000))
		assert.Equal(t, props["MemoryMax"], uint64(32*1024*1024))
		assert.DeepEqual(t, props["IOReadBandwidthMax"], [][]any{{"/dev/null", uint64(1024 * 1024)}})

		assert.NilError(t, p.InstanceStart(ctx, resourceUnit))
		props, err = p.UnitTypePropertiesForTest(ctx, resourceUnit, "Service")
		assert.NilError(t, err)
		assert.DeepEqual(t, props["IOReadBandwidthMax"], [][]any{{"/dev/null", uint64(2 * 1024 * 1024)}})
	})

	t.Run("ClearMissingActiveResources", func(t *testing.T) {
		const resourceUnit = "sablier-clear-resource-test.service"
		shared.CreateUnit(t, resourceUnit, "Enable=true\nIdleReplicas=1\nIdleCPU=0.25\nIdleMemory=32m\nIdleBlkioDeviceReadBps=/dev/null:1m")

		assert.NilError(t, p.InstanceStart(ctx, resourceUnit))
		assert.NilError(t, p.InstanceStop(ctx, resourceUnit))
		assert.NilError(t, p.InstanceStart(ctx, resourceUnit))

		props, err := p.UnitTypePropertiesForTest(ctx, resourceUnit, "Service")
		assert.NilError(t, err)
		assert.Equal(t, props["CPUQuotaPerSecUSec"], uint64(math.MaxUint64))
		assert.Equal(t, props["MemoryMax"], uint64(math.MaxUint64))
		assert.Equal(t, len(props["IOReadBandwidthMax"].([][]any)), 0)
	})
}
