package proxmoxlxc_test

import (
	"context"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupProxmox(t)
	p := env.provider
	ctx := t.Context()

	t.Run("InstanceList", func(t *testing.T) {
		instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
		assert.NilError(t, err)

		// The cloned container should appear in the list.
		found := false
		for _, inst := range instances {
			if inst.Name == env.name {
				found = true
				assert.Equal(t, inst.Group, "test") // from "sablier-group-test" tag
				break
			}
		}
		assert.Assert(t, found, "expected container %q in instance list", env.name)
	})

	t.Run("InstanceGroups", func(t *testing.T) {
		groups, err := p.InstanceGroups(ctx)
		assert.NilError(t, err)

		names, ok := groups["test"]
		assert.Assert(t, ok, "expected group 'test' to exist")

		found := false
		for _, n := range names {
			if n == env.name {
				found = true
				break
			}
		}
		assert.Assert(t, found, "expected container %q in group 'test'", env.name)
	})

	t.Run("StartAndInspect", func(t *testing.T) {
		// Container should be stopped initially (freshly cloned).
		info, err := p.InstanceInspect(ctx, env.name)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)

		// Start the container.
		err = p.InstanceStart(ctx, env.name)
		assert.NilError(t, err)

		// Poll InstanceInspect until ready (task completion + IP assignment).
		var ready bool
		for i := 0; i < 30; i++ {
			info, err = p.InstanceInspect(ctx, env.name)
			assert.NilError(t, err)

			if info.Status == sablier.InstanceStatusReady {
				ready = true
				break
			}
			t.Logf("inspect attempt %d: status=%s", i+1, info.Status)
			time.Sleep(2 * time.Second)
		}
		assert.Assert(t, ready, "expected container to become ready, last status: %s", info.Status)
		assert.Equal(t, info.Name, env.name)
	})

	t.Run("Stop", func(t *testing.T) {
		err := p.InstanceStop(ctx, env.name)
		assert.NilError(t, err)

		info, err := p.InstanceInspect(ctx, env.name)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
	})

	t.Run("InstanceEvents", func(t *testing.T) {
		// Start the container first.
		err := p.InstanceStart(ctx, env.name)
		assert.NilError(t, err)

		// Wait until it's running.
		for i := 0; i < 30; i++ {
			info, err := p.InstanceInspect(ctx, env.name)
			assert.NilError(t, err)
			if info.Status == sablier.InstanceStatusReady {
				break
			}
			time.Sleep(2 * time.Second)
		}

		// Start the event stream with a cancelable context.
		eventsCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		stream := p.InstanceEvents(eventsCtx, provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventStopped},
		})

		// Stop the container externally (simulate external stop).
		err = p.InstanceStop(ctx, env.name)
		assert.NilError(t, err)

		// Wait for the notification.
		select {
		case info, ok := <-stream.Events:
			assert.Assert(t, ok, "events channel closed unexpectedly")
			assert.Equal(t, info.Info.Name, env.name)
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for stop notification")
		}
	})
}
