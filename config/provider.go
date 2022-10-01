package config

import (
	"fmt"
)

// Provider holds the provider description
// It can be either docker, swarm or kubernetes
type Provider struct {
	Name string `mapstructure:"NAME" yaml:"provider,omitempty"`
}

var providers = []string{"docker", "swarm", "kubernetes"}

func NewProviderConfig() Provider {
	return Provider{
		Name: "docker",
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
