package nomad_test

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

var jobSeq atomic.Int64

func uniqueJobID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano()%1_000_000, jobSeq.Add(1))
}

// waitInfo polls InstanceInspect until the instance matches the condition.
func waitInfo(t *testing.T, p *nomad.Provider, name, want string, match func(sablier.InstanceInfo) bool, timeout time.Duration) sablier.InstanceInfo {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		info, err := p.InstanceInspect(t.Context(), name)
		assert.NilError(t, err)
		if match(info) {
			return info
		}
		if time.Now().After(deadline) {
			t.Fatalf("instance %q is not %s within %s (last status %q, message %q)", name, want, timeout, info.Status, info.Message)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitStatus(t *testing.T, p *nomad.Provider, name string, want sablier.InstanceStatus, timeout time.Duration) sablier.InstanceInfo {
	t.Helper()
	return waitInfo(t, p, name, string(want), func(info sablier.InstanceInfo) bool { return info.Status == want }, timeout)
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
		case <-deadline:
			t.Fatalf("no %s event for %q within %s", want, name, timeout)
		}
	}
}

// waitStreamLive registers throwaway jobs until the event stream reports one of them.
// A job registered before the stream reads its first job list gives no event, so it retries.
func waitStreamLive(t *testing.T, nc *nomadContainer, events <-chan sablier.InstanceEvent) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		name := nc.registerJob(t, uniqueJobID("sablier-it-probe"), 0, enabledMeta, time.Second)
		timeout := time.After(2 * time.Second)
		for waiting := true; waiting; {
			select {
			case ev, ok := <-events:
				if !ok {
					t.Fatal("event stream closed")
				}
				if ev.Type == provider.InstanceEventCreated && ev.Info.Name == name {
					return
				}
			case <-timeout:
				waiting = false
			}
		}
	}
	t.Fatal("the event stream did not report a new job within 1m")
}

// waitAllocsStopped waits until no allocation of the job is pending or running.
func waitAllocsStopped(t *testing.T, nc *nomadContainer, jobID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		allocs, _, err := nc.client.Jobs().Allocations(jobID, false, nil)
		assert.NilError(t, err)
		live := slices.ContainsFunc(allocs, func(a *api.AllocationListStub) bool {
			return a.ClientStatus == api.AllocClientStatusPending || a.ClientStatus == api.AllocClientStatusRunning
		})
		if !live {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %q still has live allocations after %s", jobID, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// waitDeploymentRunning waits until the latest deployment of the job is running.
func waitDeploymentRunning(t *testing.T, nc *nomadContainer, jobID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		d, _, err := nc.client.Jobs().LatestDeployment(jobID, nil)
		assert.NilError(t, err)
		if d != nil && d.Status == api.DeploymentStatusRunning {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %q has no running deployment after %s", jobID, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func TestNomadProvider_Integration(t *testing.T) {
	nc := setupNomad(t)
	p := nc.provider(t)
	ctx := t.Context()

	t.Run("ListAndInspect", func(t *testing.T) {
		name := nc.registerJob(t, uniqueJobID("sablier-it-list"), 0, map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "test",
		}, time.Second)

		instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
		assert.NilError(t, err)
		i := slices.IndexFunc(instances, func(c sablier.InstanceConfiguration) bool { return c.Name == name })
		assert.Assert(t, i >= 0, "expected %q in the instance list", name)
		assert.DeepEqual(t, instances[i].Groups, []string{"test"})
		assert.Equal(t, instances[i].Enabled, "true")

		running, err := p.InstanceList(ctx, provider.InstanceListOptions{All: false})
		assert.NilError(t, err)
		assert.Assert(t, !slices.ContainsFunc(running, func(c sablier.InstanceConfiguration) bool { return c.Name == name }), "a task group at count 0 is not running")

		groups, err := p.InstanceGroups(ctx)
		assert.NilError(t, err)
		assert.Assert(t, slices.Contains(groups["test"], name), "expected %q in group test", name)

		info, err := p.InstanceInspect(ctx, name)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
		assert.Equal(t, info.Provider, sablier.ProviderNomad)
		assert.Equal(t, info.Nomad.TaskGroup, "web")
	})

	t.Run("StartStopWithEvents", func(t *testing.T) {
		name := nc.registerJob(t, uniqueJobID("sablier-it-events"), 0, enabledMeta, time.Second)
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stream := p.InstanceEvents(streamCtx, provider.InstanceEventsOptions{})
		waitStreamLive(t, nc, stream.Events)

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

	t.Run("StopDuringDeployment", func(t *testing.T) {
		// A long minimum healthy time keeps the deployment of the scale up running.
		jobID := uniqueJobID("sablier-it-deploy")
		name := nc.registerJob(t, jobID, 0, enabledMeta, 5*time.Minute)

		assert.NilError(t, p.InstanceStart(ctx, name))
		waitDeploymentRunning(t, nc, jobID, time.Minute)
		waitInfo(t, p, name, "running and not healthy", func(info sablier.InstanceInfo) bool {
			return info.Status == sablier.InstanceStatusStarting && info.Message != ""
		}, 90*time.Second)

		assert.NilError(t, p.InstanceStop(ctx, name))
		waitStatus(t, p, name, sablier.InstanceStatusStopped, 60*time.Second)
		waitAllocsStopped(t, nc, jobID, time.Minute)
	})

	t.Run("StoppedJobIsRegisteredAgain", func(t *testing.T) {
		jobID := uniqueJobID("sablier-it-stopped")
		name := nc.registerJob(t, jobID, 1, enabledMeta, time.Second)
		waitStatus(t, p, name, sablier.InstanceStatusReady, 90*time.Second)

		_, _, err := nc.client.Jobs().Deregister(jobID, false, nil)
		assert.NilError(t, err)
		waitAllocsStopped(t, nc, jobID, time.Minute)
		info := waitStatus(t, p, name, sablier.InstanceStatusStopped, 30*time.Second)
		assert.Equal(t, info.Message, "job is stopped")

		assert.NilError(t, p.InstanceStart(ctx, name))
		waitStatus(t, p, name, sablier.InstanceStatusReady, 90*time.Second)

		assert.NilError(t, p.InstanceStop(ctx, name))
		waitStatus(t, p, name, sablier.InstanceStatusStopped, 60*time.Second)
		waitAllocsStopped(t, nc, jobID, time.Minute)
	})
}
