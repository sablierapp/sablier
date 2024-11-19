package sablier

type InstanceStatus string

const (
	InstanceStopped  InstanceStatus = "stopped"
	InstanceStarting InstanceStatus = "starting"
	InstanceRunning  InstanceStatus = "running"
	InstanceError    InstanceStatus = "error"
)

// Instance holds the data representing an instance status
type Instance struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name string

	Group string

	Kind string

	CurrentReplicas uint32
	DesiredReplicas uint32

	Status InstanceStatus
}
