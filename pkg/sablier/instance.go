package sablier

import (
	"context"
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
	// Whether the instance is enabled or not
	// To "enable" an instance is to register it through labels or annotations
	// depending on the provider.
	Enabled bool `json:"enabled"`
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name            string `json:"name"`
	Group           string `json:"group"`
	DesiredReplicas uint32 `json:"desiredReplicas"`
}

func (s *Sablier) InstancesInfo(ctx context.Context) []InstanceInfoWithError {
	s.pmu.RLock()
	defer s.pmu.RUnlock()
	return s.NewSessionInfo(ctx, s.promises).Instances
}
