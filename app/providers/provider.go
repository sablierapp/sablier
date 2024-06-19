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
	Start(ctx context.Context, name string) (instance.State, error)
	Stop(ctx context.Context, name string) (instance.State, error)
	GetState(ctx context.Context, name string) (instance.State, error)
	GetGroups(ctx context.Context) (map[string][]string, error)

	NotifyInstanceStopped(ctx context.Context, instance chan<- string)
}

func NewProvider(config config.Provider) (Provider, error) {
	if err := config.IsValid(); err != nil {
		return nil, err
	}

	switch config.Name {
	case "swarm", "docker_swarm":
		return NewDockerSwarmProvider()
	case "docker":
		return NewDockerClassicProvider()
	case "kubernetes":
		return NewKubernetesProvider(config.Kubernetes)
	case "nomad":
		return NewNomadProvider()
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}
