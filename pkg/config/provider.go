package config

import (
	"fmt"
)

// Provider holds the provider configurations
type Provider struct {
	// The provider name to use
	// It can be either docker, swarm, kubernetes, podman or proxmox_lxc. Defaults to "docker"
	Name              string `mapstructure:"NAME" yaml:"name,omitempty" default:"docker"`
	AutoStopOnStartup bool   `yaml:"auto-stop-on-startup,omitempty" default:"true"`
	Kubernetes        Kubernetes
	Podman            Podman
	Docker            Docker
	ProxmoxLXC        ProxmoxLXC `mapstructure:"PROXMOX_LXC" yaml:"proxmox-lxc,omitempty"`
}

type Kubernetes struct {
	// QPS limit for  K8S API access client-side throttle
	QPS float32 `mapstructure:"QPS" yaml:"QPS" default:"5"`
	// Maximum burst for client-side throttle
	Burst int `mapstructure:"BURST" yaml:"Burst" default:"10"`
	// Delimiter used for namespace/resource type/name resolution. Defaults to "_" for backward compatibility. But you should use "/" or ".".
	Delimiter string `mapstructure:"DELIMITER" yaml:"Delimiter" default:"_"`
}

type Podman struct {
	// Uri is the URI to connect to the Podman service.
	//
	// A valid URI connection should be scheme://
	// For example tcp://localhost:<port>
	// or unix:///run/podman/podman.sock
	// or ssh://<user>@<host>[:port]/run/podman/podman.sock
	// You can set the Uri to empty to use the CONTAINER_HOST environment variable instead.
	Uri string `mapstructure:"URI" yaml:"uri,omitempty" default:"unix:///run/podman/podman.sock"`
}

type Docker struct {
	Strategy string `mapstructure:"STRATEGY" yaml:"strategy,omitempty" default:"stop"`
}

// ProxmoxLXC holds the Proxmox VE LXC provider configuration
type ProxmoxLXC struct {
	// URL is the Proxmox VE API endpoint (e.g. "https://proxmox:8006/api2/json")
	URL string `mapstructure:"URL" yaml:"url,omitempty"`
	// TokenID is the API token identifier (e.g. "root@pam!sablier")
	TokenID string `mapstructure:"TOKEN_ID" yaml:"token-id,omitempty"`
	// TokenSecret is the API token secret UUID
	TokenSecret string `mapstructure:"TOKEN_SECRET" yaml:"token-secret,omitempty"`
	// TLSInsecure skips TLS certificate verification (useful for self-signed certificates)
	TLSInsecure bool `mapstructure:"TLS_INSECURE" yaml:"tls-insecure,omitempty" default:"false"`
}

var providers = []string{"docker", "docker_swarm", "swarm", "kubernetes", "podman", "proxmox_lxc"}
var dockerStrategies = []string{"stop", "pause"}

func NewProviderConfig() Provider {
	return Provider{

		Name: "docker",
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
	for _, s := range dockerStrategies {
		if s == docker.Strategy {
			return nil
		}
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
