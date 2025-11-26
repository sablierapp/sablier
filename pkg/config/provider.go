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
	Nomad             Nomad
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

type Nomad struct {
	// Address is the HTTP address of the Nomad server.
	// Defaults to http://127.0.0.1:4646
	// Can also be set via the NOMAD_ADDR environment variable.
	Address string `mapstructure:"ADDRESS" yaml:"address,omitempty" default:"http://127.0.0.1:4646"`
	// Token is the secret ID of an ACL token for authentication.
	// Can also be set via the NOMAD_TOKEN environment variable.
	Token string `mapstructure:"TOKEN" yaml:"token,omitempty"`
	// Namespace is the target namespace for queries.
	// Can also be set via the NOMAD_NAMESPACE environment variable.
	Namespace string `mapstructure:"NAMESPACE" yaml:"namespace,omitempty" default:"default"`
	// Region is the target region for queries.
	// Can also be set via the NOMAD_REGION environment variable.
	Region string `mapstructure:"REGION" yaml:"region,omitempty"`
}

var providers = []string{"docker", "docker_swarm", "swarm", "kubernetes", "podman", "nomad"}

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
		Nomad: Nomad{
			Address:   "http://127.0.0.1:4646",
			Namespace: "default",
			Token:     "",
			Region:    "",
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
