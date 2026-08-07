package provider

type InstanceListOptions struct {
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
