package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func trackedJob(id string, count int, meta map[string]string) *api.Job {
	job, _ := testJob(count, meta)
	job.ID = new(id)
	job.Name = new(id)
	return job
}

func eventTypes(events []sablier.InstanceEvent) []provider.InstanceEventType {
	types := make([]provider.InstanceEventType, 0, len(events))
	for _, e := range events {
		types = append(types, e.Type)
	}
	return types
}

func TestTracker(t *testing.T) {
	enabled := map[string]string{"sablier.enable": "true"}

	t.Run("seeded job emits nothing when observed unchanged", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 0, enabled))

		events := tr.observeJob(trackedJob("whoami", 0, enabled))

		assert.Equal(t, len(events), 0)
	})

	t.Run("unknown enabled job emits created", func(t *testing.T) {
		tr := newTracker("default")

		events := tr.observeJob(trackedJob("whoami", 0, enabled))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventCreated})
		assert.Equal(t, events[0].Info.Name, "whoami/web")
		assert.Equal(t, events[0].Info.Status, sablier.InstanceStatusStopped)
		assert.Equal(t, events[0].Info.Provider, sablier.ProviderNomad)
		assert.DeepEqual(t, events[0].Info.Groups, []string{"default"})
	})

	t.Run("unknown job without the enable label emits nothing", func(t *testing.T) {
		tr := newTracker("default")

		events := tr.observeJob(trackedJob("whoami", 1, nil))

		assert.Equal(t, len(events), 0)
	})

	t.Run("scaling from zero emits started", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 0, enabled))

		events := tr.observeJob(trackedJob("whoami", 1, enabled))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventStarted})
		assert.Equal(t, events[0].Info.Name, "whoami/web")
		assert.Equal(t, events[0].Info.Status, sablier.InstanceStatusStarting)
		assert.Equal(t, events[0].Info.Enabled, "true")
	})

	t.Run("scaling to zero emits stopped", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 2, enabled))

		events := tr.observeJob(trackedJob("whoami", 0, enabled))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventStopped})
		assert.Equal(t, events[0].Info.Status, sablier.InstanceStatusStopped)
	})

	t.Run("changing the count above zero emits nothing", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 1, enabled))

		events := tr.observeJob(trackedJob("whoami", 3, enabled))

		assert.Equal(t, len(events), 0)
	})

	t.Run("stopping the job emits stopped and running it again emits started", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 1, enabled))

		stopped := trackedJob("whoami", 1, enabled)
		stopped.Stop = new(true)
		events := tr.observeJob(stopped)
		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventStopped})

		events = tr.observeJob(trackedJob("whoami", 1, enabled))
		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventStarted})
	})

	t.Run("changing the labels emits updated", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 0, enabled))

		events := tr.observeJob(trackedJob("whoami", 0, map[string]string{"sablier.enable": "true", "sablier.group": "team-a"}))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventUpdated})
		assert.DeepEqual(t, events[0].Info.Groups, []string{"team-a"})
	})

	t.Run("removing the enable label emits updated without groups", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 0, enabled))

		events := tr.observeJob(trackedJob("whoami", 0, nil))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventUpdated})
		assert.Equal(t, len(events[0].Info.Groups), 0)
		assert.Equal(t, events[0].Info.Enabled, "")
	})

	t.Run("scaling and relabeling at once emits both events", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 0, enabled))

		events := tr.observeJob(trackedJob("whoami", 1, map[string]string{"sablier.enable": "true", "sablier.group": "team-a"}))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventUpdated, provider.InstanceEventStarted})
	})

	t.Run("task group removed from the job emits removed", func(t *testing.T) {
		tr := newTracker("default")
		job := trackedJob("whoami", 0, enabled)
		job.TaskGroups = append(job.TaskGroups, &api.TaskGroup{Name: new("db"), Count: new(0), Meta: enabled})
		tr.seed(job)

		events := tr.observeJob(trackedJob("whoami", 0, enabled))

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventRemoved})
		assert.Equal(t, events[0].Info.Name, "whoami/db")
	})

	t.Run("forgetting a job emits removed once per group", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("whoami", 1, enabled))

		events := tr.forgetJob("whoami")
		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{provider.InstanceEventRemoved})
		assert.Equal(t, events[0].Info.Name, "whoami/web")

		assert.Equal(t, len(tr.forgetJob("whoami")), 0)
	})

	t.Run("reconcile observes every job and forgets the missing ones", func(t *testing.T) {
		tr := newTracker("default")
		tr.seed(trackedJob("gone", 1, enabled))
		tr.seed(trackedJob("kept", 0, enabled))

		events := tr.reconcile([]*api.Job{trackedJob("kept", 1, enabled), trackedJob("new", 0, enabled)})

		assert.DeepEqual(t, eventTypes(events), []provider.InstanceEventType{
			provider.InstanceEventStarted, // kept: 0 -> 1
			provider.InstanceEventCreated, // new
			provider.InstanceEventRemoved, // gone
		})
	})
}
