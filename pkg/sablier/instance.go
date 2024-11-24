package sablier

import (
	"time"
)

type InstanceStatus string

const (
	InstanceDown     InstanceStatus = "down"
	InstanceStarting InstanceStatus = "starting"
	InstanceReady    InstanceStatus = "ready"
	InstanceError    InstanceStatus = "error"
)

type InstanceInfo struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name            string
	CurrentReplicas uint32
	DesiredReplicas uint32
	Status          InstanceStatus
	StartedAt       time.Time
}

type InstanceConfig struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name            string
	Group           string
	DesiredReplicas uint32
}
