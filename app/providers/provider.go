package providers

import (
	"context"
	"fmt"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/config"
)

const enableLabel = "sablier.enable"
const groupLabel = "sablier.group"
const defaultGroupValue = "default"

type Provider interface {
	Start(name string) (instance.State, error)
	Stop(name string) (instance.State, error)
	GetState(name string) (instance.State, error)
	GetGroups() (map[string][]string, error)
	GetManagedContainers() ([]string, error)

	NotifyInstanceStopped(ctx context.Context, instance chan<- string)
}

func NewProvider(config config.Provider) (Provider, error) {
	switch {
	case config.Name == "swarm":
		return NewDockerSwarmProvider()
	case config.Name == "docker":
		return NewDockerClassicProvider()
	case config.Name == "kubernetes":
		return NewKubernetesProvider()
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}
