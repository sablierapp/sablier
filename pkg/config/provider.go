package config

import (
	"fmt"
)

// Provider holds the provider configurations
type Provider struct {
	// The provider name to use
	// It can be either docker, swarm or kubernetes. Defaults to "docker"
	Name              string `mapstructure:"NAME" yaml:"name,omitempty" default:"docker"`
	AutoStopOnStartup bool   `yaml:"auto-stop-on-startup,omitempty" default:"true"`
	Kubernetes        Kubernetes
	Podman            Podman
	DigitalOcean      DigitalOcean
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

type DigitalOcean struct {
	// Token is the Digital Ocean API token for authentication
	Token string `mapstructure:"TOKEN" yaml:"token,omitempty"`
	// Region is the Digital Ocean region. Defaults to "nyc1"
	Region string `mapstructure:"REGION" yaml:"region,omitempty" default:"nyc1"`
}

var providers = []string{"docker", "docker_swarm", "swarm", "kubernetes", "podman", "digitalocean"}

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
		DigitalOcean: DigitalOcean{
			Region: "nyc1",
		},
	}
}

func (provider Provider) IsValid() error {
	for _, p := range providers {
		if p == provider.Name {
			return nil
		}
	}
	return fmt.Errorf("unrecognized provider %s. providers available: %v", provider.Name, providers)
}

func GetProviders() []string {
	return providers
}
