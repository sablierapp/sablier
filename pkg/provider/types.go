package provider

type InstanceListOptions struct {
	All bool
}

type EventType string

const (
	EventTypeInstance EventType = "instance"
)

type EventAction string

const (
	EventActionUnknown  EventAction = "unknown"
	EventActionCreated  EventAction = "created"
	EventActionPending  EventAction = "pending"
	EventActionRunning  EventAction = "running"
	EventActionStopping EventAction = "stopping"
	EventActionStopped  EventAction = "stopped"
	EventActionFailed   EventAction = "failed"
	EventActionRemoved  EventAction = "removed"
)

type Event struct {
	Type          EventType
	Action        EventAction
	InstanceName  string
	InstanceGroup string
	ProviderName  string
	Event         any
}
