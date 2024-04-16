package config

import (
	"fmt"
)

// Provider holds the provider description
// It can be either docker, swarm or kubernetes
type Provider struct {
	Name       string `mapstructure:"NAME" yaml:"provider,omitempty"`
	Kubernetes Kubernetes
}

type Kubernetes struct {
	//QPS limit for  K8S API access client-side throttle
	QPS float32 `mapstructure:"QPS" yaml:"QPS" default:"5"`
	//Maximum burst for client-side throttle
	Burst int `mapstructure:"BURST" yaml:"Burst" default:"10"`
}

var providers = []string{"docker", "docker_swarm", "swarm", "kubernetes"}

func NewProviderConfig() Provider {
	return Provider{
		Name: "docker",
		Kubernetes: Kubernetes{
			QPS:   5,
			Burst: 10,
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
