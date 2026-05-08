package sablier

// DockerContainerInfo holds Docker-specific container metadata.
type DockerContainerInfo struct {
	ID     string            `json:"id"`
	Image  string            `json:"image"`
	Labels map[string]string `json:"labels,omitempty"`
}

// SwarmServiceInfo holds Docker Swarm-specific service metadata.
type SwarmServiceInfo struct {
	ID     string            `json:"id"`
	Image  string            `json:"image"`
	Labels map[string]string `json:"labels,omitempty"`
}

// KubernetesWorkloadInfo holds Kubernetes-specific workload metadata.
type KubernetesWorkloadInfo struct {
	Namespace string            `json:"namespace"`
	Kind      string            `json:"kind"`
	Image     string            `json:"image"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// PodmanContainerInfo holds Podman-specific container metadata.
type PodmanContainerInfo struct {
	ID     string            `json:"id"`
	Image  string            `json:"image"`
	Labels map[string]string `json:"labels,omitempty"`
}
