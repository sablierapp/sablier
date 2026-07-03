package config

import (
	"fmt"
	"slices"
)

// Provider holds the provider configurations.
type Provider struct {
	// Name selects the container runtime to manage workloads.
	// Accepted values: docker, swarm, kubernetes, podman, proxmox_lxc.
	// Env: SABLIER_PROVIDER_NAME
	// CLI: --provider.name
	// Default: "docker"
	Name string

	// AutoStopOnStartup stops all instances labelled with sablier.enable=true at
	// Sablier startup, ensuring a clean zero-scale state even after an unclean shutdown.
	// Env: SABLIER_PROVIDER_AUTO_STOP_ON_STARTUP
	// CLI: --provider.auto-stop-on-startup
	// Default: true
	AutoStopOnStartup bool

	// AutoStopExternallyStarted continuously stops instances with sablier.enable=true
	// that are running but were not started by Sablier itself.
	// Env: SABLIER_PROVIDER_AUTO_STOP_EXTERNALLY_STARTED
	// CLI: --provider.auto-stop-externally-started
	// Default: false
	AutoStopExternallyStarted bool

	// AutoWarmExternallyStarted continuously creates a session (with the default
	// session duration) for instances with sablier.enable=true that are running
	// but were not started by Sablier itself, instead of stopping them. The regular
	// expiration lifecycle then stops the instance once its session expires.
	// This is the non-destructive counterpart to AutoStopExternallyStarted.
	// Env: SABLIER_PROVIDER_AUTO_WARM_EXTERNALLY_STARTED
	// CLI: --provider.auto-warm-externally-started
	// Default: false
	AutoWarmExternallyStarted bool

	// RejectUnlabeledRequests rejects requests for instances that do not carry
	// the sablier.enable=true label, preventing accidental management of unlabelled workloads.
	// Env: SABLIER_PROVIDER_REJECT_UNLABELED_REQUESTS
	// CLI: --provider.reject-unlabeled-requests
	// Default: false
	RejectUnlabeledRequests bool

	// VerifyEnabledOnExpiration re-checks the sablier.enable=true label before stopping
	// an expired instance, useful when labels are managed dynamically.
	// Env: SABLIER_PROVIDER_VERIFY_ENABLED_ON_EXPIRATION
	// CLI: --provider.verify-enabled-on-expiration
	// Default: false
	VerifyEnabledOnExpiration bool

	Kubernetes Kubernetes
	Podman     Podman
	Docker     Docker
	ProxmoxLXC ProxmoxLXC
}

type Kubernetes struct {
	// QPS is the maximum number of queries per second sent to the Kubernetes API server
	// for client-side rate limiting.
	// Env: SABLIER_PROVIDER_KUBERNETES_QPS
	// CLI: --provider.kubernetes.qps
	// Default: 5
	QPS float32

	// Burst is the maximum number of requests the Kubernetes client can send in a burst
	// before rate limiting kicks in.
	// Env: SABLIER_PROVIDER_KUBERNETES_BURST
	// CLI: --provider.kubernetes.burst
	// Default: 10
	Burst int

	// Delimiter separates the namespace, resource type, and name in instance identifiers.
	// Defaults to "_" for backward compatibility; prefer "/" or "." for new deployments.
	// Env: SABLIER_PROVIDER_KUBERNETES_DELIMITER
	// CLI: --provider.kubernetes.delimiter
	// Default: "_"
	Delimiter string
}

type Podman struct {
	// Uri is the connection URI for the Podman service.
	// Accepted schemes: unix://, tcp://, ssh://.
	// Leave empty to fall back to the CONTAINER_HOST environment variable.
	// Env: SABLIER_PROVIDER_PODMAN_URI
	// CLI: --provider.podman.uri
	// Default: "unix:///run/podman/podman.sock"
	Uri string
}

type Docker struct {
	// Strategy controls how containers are brought to a stopped state.
	// "stop" terminates the container process, freeing both CPU and memory.
	// "pause" suspends execution while keeping the container in memory, allowing faster restarts.
	// Env: SABLIER_PROVIDER_DOCKER_STRATEGY
	// CLI: --provider.docker.strategy
	// Default: "stop"
	Strategy string

	// HonorRestartPolicy makes Sablier honor a container's restart policy when it
	// exits successfully (exit code 0). When enabled, a container with a "no" or
	// "on-failure" policy is reported as completed (a one-shot/init container that
	// finished its job). An "always"/"unless-stopped" container that is exited was
	// stopped and is reported as stopped (Docker does not auto-restart a manually
	// stopped container). When disabled, Sablier keeps the historical behavior and
	// always reports a successfully exited container as stopped.
	//
	// Note: Docker normalizes an unset restart policy to "no", so an unset policy
	// is indistinguishable from an explicit "no" and is therefore also reported
	// as completed when this option is enabled.
	//
	// Deprecated: this option only exists to preserve backward compatibility. It
	// will be removed in v2, where honoring the restart policy becomes the
	// default behavior.
	// Env: SABLIER_PROVIDER_DOCKER_HONOR_RESTART_POLICY
	// CLI: --provider.docker.honor-restart-policy
	// Default: false
	HonorRestartPolicy bool
}

// ProxmoxLXC holds the Proxmox VE LXC provider configuration.
type ProxmoxLXC struct {
	// URL is the Proxmox VE REST API base URL (e.g. "https://proxmox:8006/api2/json").
	// Env: SABLIER_PROVIDER_PROXMOX_LXC_URL
	// CLI: --provider.proxmox-lxc.url
	// Default: ""
	URL string

	// TokenID is the Proxmox API token identifier in the form "user@realm!tokenname"
	// (e.g. "root@pam!sablier").
	// Env: SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_ID
	// CLI: --provider.proxmox-lxc.token-id
	// Default: ""
	TokenID string

	// TokenSecret is the UUID secret associated with the Proxmox API token.
	// Env: SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_SECRET
	// CLI: --provider.proxmox-lxc.token-secret
	// Default: ""
	TokenSecret string

	// TLSInsecure disables TLS certificate verification when connecting to the Proxmox API.
	// Enable only for self-signed certificates in trusted networks.
	// Env: SABLIER_PROVIDER_PROXMOX_LXC_TLS_INSECURE
	// CLI: --provider.proxmox-lxc.tls-insecure
	// Default: false
	TLSInsecure bool
}

var providers = []string{"docker", "docker_swarm", "swarm", "kubernetes", "podman", "proxmox_lxc"}
var dockerStrategies = []string{"stop", "pause"}

func NewProviderConfig() Provider {
	return Provider{
		Name:              "docker",
		AutoStopOnStartup: true,
		Kubernetes: Kubernetes{
			QPS:       5,
			Burst:     10,
			Delimiter: "_",
		},
		Podman: Podman{
			Uri: "unix:///run/podman/podman.sock",
		},
		Docker: Docker{
			Strategy: "stop",
		},
		ProxmoxLXC: ProxmoxLXC{},
	}
}

func (provider Provider) IsValid() error {
	if provider.AutoStopExternallyStarted && provider.AutoWarmExternallyStarted {
		return fmt.Errorf("provider.auto-stop-externally-started and provider.auto-warm-externally-started are mutually exclusive")
	}
	for _, p := range providers {
		if p == provider.Name {
			// Validate Docker-specific settings when using Docker provider
			if p == "docker" {
				if err := provider.Docker.IsValid(); err != nil {
					return err
				}
			}
			// Validate Proxmox LXC-specific settings
			if p == "proxmox_lxc" {
				if err := provider.ProxmoxLXC.IsValid(); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("unrecognized provider %s. providers available: %v", provider.Name, providers)
}

func (docker Docker) IsValid() error {
	if slices.Contains(dockerStrategies, docker.Strategy) {
		return nil
	}
	return fmt.Errorf("unrecognized docker strategy %s. strategies available: %v", docker.Strategy, dockerStrategies)
}

func (p ProxmoxLXC) IsValid() error {
	if p.URL == "" {
		return fmt.Errorf("proxmox_lxc provider requires a URL")
	}
	if p.TokenID == "" {
		return fmt.Errorf("proxmox_lxc provider requires a token ID")
	}
	if p.TokenSecret == "" {
		return fmt.Errorf("proxmox_lxc provider requires a token secret")
	}
	return nil
}

func GetProviders() []string {
	return providers
}
