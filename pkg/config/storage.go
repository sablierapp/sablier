package config

import "errors"

// Storage holds the state persistence configuration.
type Storage struct {
	// File is the path to a file where Sablier persists its session state across restarts.
	// Leave empty to run stateless (state is lost on restart).
	// Env: SABLIER_STORAGE_FILE
	// CLI: --storage.file
	// Default: "" (stateless)
	// Since: v1.0.0
	File string

	Kubernetes KubernetesStorage
}

// KubernetesStorage persists the session state in a Kubernetes ConfigMap, so
// it survives pod restarts and rescheduling without a persistent volume.
type KubernetesStorage struct {
	// ConfigMap is the name of the ConfigMap where Sablier persists its session
	// state. Sablier creates it when it does not exist. New and removed sessions
	// are written immediately, session refreshes at most every 30 seconds, and
	// everything once more on shutdown. Requires get/create/update on configmaps
	// in the ConfigMap namespace. Cannot be combined with storage.file.
	// Env: SABLIER_STORAGE_KUBERNETES_CONFIGMAP
	// CLI: --storage.kubernetes.configmap
	// Default: "" (disabled)
	// Since: NEXT_RELEASE
	ConfigMap string

	// Namespace is the namespace of the ConfigMap. When empty, Sablier's own
	// namespace: the POD_NAMESPACE environment variable, then the service
	// account namespace file mounted in the pod.
	// Env: SABLIER_STORAGE_KUBERNETES_NAMESPACE
	// CLI: --storage.kubernetes.namespace
	// Default: ""
	// Since: NEXT_RELEASE
	Namespace string
}

func NewStorageConfig() Storage {
	return Storage{
		File: "",
	}
}

func (s Storage) IsValid() error {
	if s.File != "" && s.Kubernetes.ConfigMap != "" {
		return errors.New("storage.file and storage.kubernetes.configmap cannot be combined")
	}
	return nil
}
