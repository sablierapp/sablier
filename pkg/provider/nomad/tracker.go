package nomad

import (
	"maps"
	"slices"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// tracker turns job specs into instance lifecycle events by diffing each task
// group against its last known state, because Nomad only reports job registrations.
type tracker struct {
	namespace string
	groups    map[string]groupSnapshot // instance name -> last known state
}

type groupSnapshot struct {
	jobID   string
	running bool // count > 0 and the job is not stopped
	enabled bool
	meta    map[string]string
}

func newTracker(namespace string) *tracker {
	return &tracker{namespace: namespace, groups: make(map[string]groupSnapshot)}
}

func snapshotOf(job *api.Job, tg *api.TaskGroup) groupSnapshot {
	meta := groupMeta(job, tg)
	return groupSnapshot{
		jobID:   deref(job.ID),
		running: deref(tg.Count) > 0 && !deref(job.Stop),
		enabled: meta[sablier.LabelEnable] == "true",
		meta:    meta,
	}
}

// seed records the current state of a job without emitting events.
func (t *tracker) seed(job *api.Job) {
	for _, tg := range job.TaskGroups {
		t.groups[instanceName(deref(job.ID), deref(tg.Name))] = snapshotOf(job, tg)
	}
}

// observeJob records the state of a job and returns the events implied by the
// changes since the last observation. Never enabled task groups are ignored.
func (t *tracker) observeJob(job *api.Job) []sablier.InstanceEvent {
	jobID := deref(job.ID)
	var events []sablier.InstanceEvent
	seen := make(map[string]bool, len(job.TaskGroups))

	for _, tg := range job.TaskGroups {
		name := instanceName(jobID, deref(tg.Name))
		cur := snapshotOf(job, tg)
		prev, known := t.groups[name]
		t.groups[name] = cur
		seen[name] = true

		if !known {
			if cur.enabled {
				events = append(events, t.event(provider.InstanceEventCreated, name, cur))
			}
			continue
		}
		if !prev.enabled && !cur.enabled {
			continue
		}
		if !maps.Equal(prev.meta, cur.meta) {
			events = append(events, t.event(provider.InstanceEventUpdated, name, cur))
		}
		switch {
		case !prev.running && cur.running:
			events = append(events, t.event(provider.InstanceEventStarted, name, cur))
		case prev.running && !cur.running:
			events = append(events, t.event(provider.InstanceEventStopped, name, cur))
		}
	}

	// Task groups dropped from the job spec.
	var removed []string
	for name, snap := range t.groups {
		if snap.jobID == jobID && !seen[name] {
			removed = append(removed, name)
		}
	}
	events = append(events, t.forget(removed)...)
	return events
}

// forgetJob drops every task group of a job and reports them as removed.
func (t *tracker) forgetJob(jobID string) []sablier.InstanceEvent {
	var names []string
	for name, snap := range t.groups {
		if snap.jobID == jobID {
			names = append(names, name)
		}
	}
	return t.forget(names)
}

func (t *tracker) forget(names []string) []sablier.InstanceEvent {
	slices.Sort(names)
	var events []sablier.InstanceEvent
	for _, name := range names {
		snap := t.groups[name]
		delete(t.groups, name)
		if snap.enabled {
			events = append(events, t.event(provider.InstanceEventRemoved, name, snap))
		}
	}
	return events
}

// reconcile observes every given job and forgets the jobs that are gone, which
// reports what changed while the event stream was disconnected.
func (t *tracker) reconcile(jobs []*api.Job) []sablier.InstanceEvent {
	var events []sablier.InstanceEvent
	current := make(map[string]bool, len(jobs))
	for _, job := range jobs {
		current[deref(job.ID)] = true
		events = append(events, t.observeJob(job)...)
	}

	var gone []string
	for _, snap := range t.groups {
		if !current[snap.jobID] && !slices.Contains(gone, snap.jobID) {
			gone = append(gone, snap.jobID)
		}
	}
	slices.Sort(gone)
	for _, jobID := range gone {
		events = append(events, t.forgetJob(jobID)...)
	}
	return events
}

func (t *tracker) event(kind provider.InstanceEventType, name string, snap groupSnapshot) sablier.InstanceEvent {
	info := sablier.InstanceInfo{
		Name:            name,
		DesiredReplicas: max(sablier.ScaleConfigFromLabels(snap.meta).Active.Replicas, 1),
		Status:          sablier.InstanceStatusStopped,
		Provider:        sablier.ProviderNomad,
	}
	if snap.running && kind != provider.InstanceEventRemoved {
		info.Status = sablier.InstanceStatusStarting
	}
	sablier.PopulateEnabledAndGroup(&info, snap.meta)

	parsed, _ := parseName(name)
	info.Nomad = &sablier.NomadTaskGroupInfo{
		Namespace: t.namespace,
		JobID:     snap.jobID,
		TaskGroup: parsed.Group,
		Meta:      snap.meta,
	}
	return sablier.InstanceEvent{Type: kind, Info: info}
}
