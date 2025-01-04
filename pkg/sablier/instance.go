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
	Name            string         `json:"name"`
	CurrentReplicas uint32         `json:"currentReplicas"`
	DesiredReplicas uint32         `json:"desiredReplicas"`
	Status          InstanceStatus `json:"status"`
	StartedAt       time.Time      `json:"startedAt"`
	ExpiresAt       time.Time      `json:"expiresAt"`
}

type InstanceConfig struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name            string
	Group           string
	DesiredReplicas uint32
}
