package sablier

type InstanceStatus string

const (
	InstanceStopped  InstanceStatus = "stopped"
	InstanceStarting InstanceStatus = "starting"
	InstanceRunning  InstanceStatus = "running"
	InstanceError    InstanceStatus = "error"
)

type InstanceInfo struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name            string
	CurrentReplicas uint32
	DesiredReplicas uint32
	Status          InstanceStatus
}

type InstanceConfig struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name            string
	Group           string
	DesiredReplicas uint32
}
