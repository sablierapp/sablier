package config

import (
	"fmt"
	"time"
)

// Provider holds the provider configurations
type Provider struct {
	// The provider name to use
	// It can be either docker, swarm or kubernetes. Defaults to "docker"
	Name              string `mapstructure:"NAME" yaml:"name,omitempty" default:"docker"`
	AutoStopOnStartup bool   `yaml:"auto-stop-on-startup,omitempty" default:"true"`
	Kubernetes        Kubernetes
	Docker            Docker
}

type Kubernetes struct {
	//QPS limit for  K8S API access client-side throttle
	QPS float32 `mapstructure:"QPS" yaml:"QPS" default:"5"`
	//Maximum burst for client-side throttle
	Burst int `mapstructure:"BURST" yaml:"Burst" default:"10"`
	//Delimiter used for namespace/resource type/name resolution. Defaults to "_" for backward compatibility. But you should use "/" or ".".
	Delimiter string `mapstructure:"DELIMITER" yaml:"Delimiter" default:"_"`
}

type Docker struct {
	// Initial delay between reconnection attempts to Docker
	ReconnectInitialDelay time.Duration `mapstructure:"RECONNECT_INITIAL_DELAY" yaml:"reconnectInitialDelay" default:"1s"`
	// Maximum delay between reconnection attempts to Docker
	ReconnectMaxDelay time.Duration `mapstructure:"RECONNECT_MAX_DELAY" yaml:"reconnectMaxDelay" default:"30s"`
	// Maximum number of reconnection attempts (0 = unlimited)
	ReconnectMaxAttempts int `mapstructure:"RECONNECT_MAX_ATTEMPTS" yaml:"reconnectMaxAttempts" default:"0"`
}

var providers = []string{"docker", "docker_swarm", "swarm", "kubernetes"}

func NewProviderConfig() Provider {
	return Provider{

		Name: "docker",
		Kubernetes: Kubernetes{
			QPS:       5,
			Burst:     10,
			Delimiter: "_", //Delimiter used for namespace/resource type/name resolution. Defaults to "_" for backward compatibility. But you should use "/" or ".".
		},
		Docker: Docker{
			ReconnectInitialDelay: 1 * time.Second,
			ReconnectMaxDelay:     30 * time.Second,
			ReconnectMaxAttempts:  0, // 0 means unlimited
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
