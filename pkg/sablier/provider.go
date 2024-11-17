package sablier

type EventAction string

const (
	// EventActionCreate describes when a workload has been created
	EventActionCreate EventAction = "create"

	// EventActionRemove describes when a workload has been destroyed
	EventActionRemove EventAction = "destroy"

	// EventActionReady describes when a workload is ready to handle traffic
	EventActionReady EventAction = "ready"

	// EventActionStart describes when a workload is started but not necessarily ready

	EventActionStart EventAction = "start"

	// EventActionStop describes when a workload is stopped
	EventActionStop EventAction = "stop"
)

type Message struct {
	Name   string
	Group  string
	Action EventAction
}
