package sablier

import (
	"log/slog"
	"time"
)

type InstanceStatus string

const (
	InstanceStatusStopped  InstanceStatus = "stopped"
	InstanceStatusStarting InstanceStatus = "starting"
	InstanceStatusReady    InstanceStatus = "ready"
	InstanceStatusError    InstanceStatus = "error"
)

// ProviderType identifies the infrastructure provider that manages an instance.
type ProviderType = string

const (
	ProviderDocker     ProviderType = "docker"
	ProviderSwarm      ProviderType = "swarm"
	ProviderKubernetes ProviderType = "kubernetes"
	ProviderPodman     ProviderType = "podman"
)

type InstanceInfo struct {
	Name            string                  `json:"name"`
	CurrentReplicas int32                   `json:"currentReplicas"`
	DesiredReplicas int32                   `json:"desiredReplicas"`
	Status          InstanceStatus          `json:"status"`
	Group           string                  `json:"group,omitempty"`
	Enabled         string                  `json:"enabled,omitempty"`
	Message         string                  `json:"message,omitempty"`
	Provider        ProviderType            `json:"provider,omitempty"`
	Docker          *DockerContainerInfo    `json:"docker,omitempty"`
	Swarm           *SwarmServiceInfo       `json:"swarm,omitempty"`
	Kubernetes      *KubernetesWorkloadInfo `json:"kubernetes,omitempty"`
	Podman          *PodmanContainerInfo    `json:"podman,omitempty"`

	// ReadyAfter is the minimum duration to wait after the instance first
	// reports ready before Sablier considers it truly ready. Set via the
	// sablier.ready-after label (e.g. "30s"). Zero means no extra wait.
	ReadyAfter time.Duration `json:"readyAfter,omitempty"`
	// ReadyAt records when the instance first transitioned to InstanceStatusReady.
	// It is set internally by Sablier and is never populated by a provider.
	ReadyAt *time.Time `json:"readyAt,omitempty"`

	// RunningHours is a daily keep-warm window in local time, parsed from
	// the sablier.running-hours label (format: HH:MM-HH:MM).
	RunningHours string `json:"runningHours,omitempty"`
}

type InstanceConfiguration struct {
	Name    string
	Group   string
	Enabled string
}

func (instance InstanceInfo) IsReady() bool {
	if instance.Status != InstanceStatusReady {
		return false
	}
	if instance.ReadyAfter == 0 || instance.ReadyAt == nil {
		return true
	}
	return time.Since(*instance.ReadyAt) >= instance.ReadyAfter
}

// PopulateEnabledAndGroup reads the sablier.enable and sablier.group labels from
// labels and writes the results into info. Centralising this logic avoids
// duplicating the same map lookups in every provider's Inspect implementation.
func PopulateEnabledAndGroup(info *InstanceInfo, labels map[string]string) {
	info.Enabled = labels["sablier.enable"]
	if info.Enabled == "true" {
		if g := labels["sablier.group"]; g != "" {
			info.Group = g
		} else {
			info.Group = "default"
		}
	}
	if v := labels["sablier.ready-after"]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			info.ReadyAfter = d
		} else {
			slog.Warn("invalid sablier.ready-after label value, ignoring",
				slog.String("instance", info.Name),
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels["sablier.running-hours"]; v != "" {
		if _, err := ParseRunningHours(v); err == nil {
			info.RunningHours = v
		} else {
			slog.Warn("invalid sablier.running-hours label value, ignoring",
				slog.String("instance", info.Name),
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
}
