package sablier

import (
	"log/slog"
	"strconv"
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

	// ScaleConfig configures resource-based scale mode for this instance.
	// When present, Sablier throttles CPU/memory instead of stopping the container.
	ScaleConfig *ScaleConfig `json:"scaleConfig,omitempty"`
}

// ScaleConfig defines the idle and active resource profiles used in scale mode.
// In scale mode, the container keeps running but its resources are adjusted:
// idle resources are applied when the session expires, active resources when
// a new session is requested.
type ScaleConfig struct {
	Idle   ResourceProfile `json:"idle,omitempty"`
	Active ResourceProfile `json:"active,omitempty"`
}

// ResourceProfile holds the CPU and memory limits for a single resource profile.
type ResourceProfile struct {
	// Replicas is the desired replica count for this profile.
	// For idle: 0 (default) stops the workload; ≥ 1 keeps it running with
	// throttled resources (resource scaling mode).
	// For active: defaults to 1.
	Replicas int32 `json:"replicas,omitempty"`
	// CPU is the CPU limit (e.g. "0.5" for Docker/Swarm, "500m" for Kubernetes).
	CPU string `json:"cpu,omitempty"`
	// Memory is the memory limit (e.g. "128m" for Docker/Swarm, "128Mi" for Kubernetes).
	Memory string `json:"memory,omitempty"`
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

// ScaleConfigFromLabels extracts a ScaleConfig from the given label map.
// Returns nil if none of the scale labels (sablier.idle.{cpu,memory,replicas},
// sablier.active.{cpu,memory,replicas}) are present.
//
// Defaults:
//   - Idle.Replicas = 0 (workload is stopped when idle)
//   - Active.Replicas = 1 (workload runs with a single replica when active)
//
// Resource scaling (CPU/memory throttling instead of stopping) is only applied
// when Idle.Replicas ≥ 1.
func ScaleConfigFromLabels(labels map[string]string) *ScaleConfig {
	idleCPU := labels["sablier.idle.cpu"]
	idleMemory := labels["sablier.idle.memory"]
	activeCPU := labels["sablier.active.cpu"]
	activeMemory := labels["sablier.active.memory"]
	_, hasIdleReplicas := labels["sablier.idle.replicas"]
	_, hasActiveReplicas := labels["sablier.active.replicas"]

	if idleCPU == "" && idleMemory == "" && !hasIdleReplicas &&
		activeCPU == "" && activeMemory == "" && !hasActiveReplicas {
		return nil
	}

	idleReplicas := int32(0) // default: stop the workload
	if v, ok := labels["sablier.idle.replicas"]; ok {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n >= 0 {
			idleReplicas = int32(n)
		}
	}

	activeReplicas := int32(1) // default: one running replica
	if v, ok := labels["sablier.active.replicas"]; ok {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n >= 0 {
			activeReplicas = int32(n)
		}
	}

	return &ScaleConfig{
		Idle: ResourceProfile{
			Replicas: idleReplicas,
			CPU:      idleCPU,
			Memory:   idleMemory,
		},
		Active: ResourceProfile{
			Replicas: activeReplicas,
			CPU:      activeCPU,
			Memory:   activeMemory,
		},
	}
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
	info.ScaleConfig = ScaleConfigFromLabels(labels)
}
