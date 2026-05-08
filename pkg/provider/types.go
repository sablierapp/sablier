package provider

type InstanceListOptions struct {
	All bool
}

// InstanceEventType identifies which kind of instance state change to subscribe to.
type InstanceEventType string

const (
	// InstanceEventStopped fires when an instance transitions to stopped / scaled-to-zero.
	InstanceEventStopped InstanceEventType = "stopped"
)

// InstanceEventsOptions controls which events InstanceEvents streams.
type InstanceEventsOptions struct {
	// Types lists the event types to receive.
	// An empty slice means all event types.
	Types []InstanceEventType
}
