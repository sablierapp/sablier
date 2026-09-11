package nomad_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// waitStatus polls InstanceInspect until the instance reaches the wanted status.
func waitStatus(t *testing.T, p *nomad.Provider, name string, want sablier.InstanceStatus, timeout time.Duration) sablier.InstanceInfo {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last sablier.InstanceInfo
	for {
		info, err := p.InstanceInspect(t.Context(), name)
		assert.NilError(t, err)
		if info.Status == want {
			return info
		}
		last = info
		if time.Now().After(deadline) {
			t.Fatalf("instance %q did not reach status %q within %s (last: %q, message %q)", name, want, timeout, last.Status, last.Message)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// waitEvent reads events until one of the wanted type for the named instance arrives.
func waitEvent(t *testing.T, events <-chan sablier.InstanceEvent, name string, want provider.InstanceEventType, timeout time.Duration) sablier.InstanceEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("event stream closed")
			}
			if ev.Info.Name == name && ev.Type == want {
				return ev
			}
			t.Logf("ignoring event %s for %s", ev.Type, ev.Info.Name)
		case <-deadline:
			t.Fatalf("no %s event for %q within %s", want, name, timeout)
		}
	}
}

func TestNomadProvider_Integration(t *testing.T) {
	nc := setupNomad(t)
	p := nc.provider(t)
	ctx := t.Context()

	name := nc.registerJob(t, fmt.Sprintf("sablier-it-%d", time.Now().UnixNano()%1_000_000_000), 0, map[string]string{
		"sablier.enable": "true",
		"sablier.group":  "test",
	})

	t.Run("InstanceList", func(t *testing.T) {
		instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
		assert.NilError(t, err)
		i := slices.IndexFunc(instances, func(c sablier.InstanceConfiguration) bool { return c.Name == name })
		assert.Assert(t, i >= 0, "expected %q in the instance list", name)
		assert.DeepEqual(t, instances[i].Groups, []string{"test"})
		assert.Equal(t, instances[i].Enabled, "true")

		running, err := p.InstanceList(ctx, provider.InstanceListOptions{All: false})
		assert.NilError(t, err)
		assert.Assert(t, !slices.ContainsFunc(running, func(c sablier.InstanceConfiguration) bool { return c.Name == name }), "a task group at count 0 is not running")
	})

	t.Run("InstanceGroups", func(t *testing.T) {
		groups, err := p.InstanceGroups(ctx)
		assert.NilError(t, err)
		assert.Assert(t, slices.Contains(groups["test"], name), "expected %q in group test", name)
	})

	t.Run("InspectStopped", func(t *testing.T) {
		info, err := p.InstanceInspect(ctx, name)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
		assert.Equal(t, info.Provider, sablier.ProviderNomad)
		assert.Equal(t, info.Nomad.TaskGroup, "web")
	})

	t.Run("StartStopWithEvents", func(t *testing.T) {
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stream := p.InstanceEvents(streamCtx, provider.InstanceEventsOptions{})

		assert.NilError(t, p.InstanceStart(ctx, name))
		waitEvent(t, stream.Events, name, provider.InstanceEventStarted, 30*time.Second)
		info := waitStatus(t, p, name, sablier.InstanceStatusReady, 90*time.Second)
		assert.Equal(t, info.CurrentReplicas, int32(1))
		assert.Equal(t, info.DesiredReplicas, int32(1))

		running, err := p.InstanceList(ctx, provider.InstanceListOptions{All: false})
		assert.NilError(t, err)
		assert.Assert(t, slices.ContainsFunc(running, func(c sablier.InstanceConfiguration) bool { return c.Name == name }), "a started task group is running")

		// Starting again is a no-op.
		assert.NilError(t, p.InstanceStart(ctx, name))

		assert.NilError(t, p.InstanceStop(ctx, name))
		waitEvent(t, stream.Events, name, provider.InstanceEventStopped, 30*time.Second)
		waitStatus(t, p, name, sablier.InstanceStatusStopped, 60*time.Second)
	})

	t.Run("StoppedJobIsRegisteredAgain", func(t *testing.T) {
		jobID := name[:len(name)-len("/web")]
		_, _, err := nc.client.Jobs().Deregister(jobID, false, nil)
		assert.NilError(t, err)

		info := waitStatus(t, p, name, sablier.InstanceStatusStopped, 30*time.Second)
		assert.Equal(t, info.Message, "job is stopped")

		assert.NilError(t, p.InstanceStart(ctx, name))
		waitStatus(t, p, name, sablier.InstanceStatusReady, 90*time.Second)

		assert.NilError(t, p.InstanceStop(ctx, name))
		waitStatus(t, p, name, sablier.InstanceStatusStopped, 60*time.Second)
	})
}
