package provider

import "errors"

// ErrNoop is returned by a provider's InstanceStop when the instance needed no
// change, for example a scale-mode container whose idle profile has
// replicas >= 1 and no idle resources. Nothing was stopped. Callers that only
// want the instance idle should treat it as success; callers that log or count
// a stop should skip that.
var ErrNoop = errors.New("nothing to do")

type InstanceListOptions struct {
	// All includes the stopped instances. If All is false, the provider must
	// return only the running instances.
	All bool
}

// InstanceEventType identifies which kind of instance lifecycle change to subscribe to.
type InstanceEventType string

const (
	// InstanceEventStopped fires when an instance transitions to stopped / scaled-to-zero.
	InstanceEventStopped InstanceEventType = "stopped"
	// InstanceEventStarted fires when an instance transitions to started / running / scaled-from-zero.
	InstanceEventStarted InstanceEventType = "started"
	// InstanceEventCreated fires when a new instance (container/service/workload) is created.
	InstanceEventCreated InstanceEventType = "created"
	// InstanceEventUpdated fires when an instance's configuration (e.g. labels) changes.
	InstanceEventUpdated InstanceEventType = "updated"
	// InstanceEventRemoved fires when an instance is permanently deleted from the provider.
	InstanceEventRemoved InstanceEventType = "removed"
)

// InstanceEventsOptions controls which events InstanceEvents streams.
type InstanceEventsOptions struct {
	// Types lists the event types to receive.
	// An empty slice means all event types.
	Types []InstanceEventType
}
