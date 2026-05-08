package sablier

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
}

type InstanceConfiguration struct {
	Name    string
	Group   string
	Enabled string
}

func (instance InstanceInfo) IsReady() bool {
	return instance.Status == InstanceStatusReady
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
}
