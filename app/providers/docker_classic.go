package providers

import (
	"context"
	"fmt"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type DockerClassicProvider struct {
	Client          client.ContainerAPIClient
	desiredReplicas int
}

func NewDockerClassicProvider() (*DockerClassicProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
		return nil, err
	}
	return &DockerClassicProvider{
		Client:          cli,
		desiredReplicas: 1,
	}, nil
}

func (provider *DockerClassicProvider) Start(name string) (instance.State, error) {
	ctx := context.Background()

	err := provider.Client.ContainerStart(ctx, name, types.ContainerStartOptions{})

	if err != nil {
		return instance.ErrorInstanceState(name, err, provider.desiredReplicas)
	}

	return instance.State{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: provider.desiredReplicas,
		Status:          instance.NotReady,
	}, err
}

func (provider *DockerClassicProvider) Stop(name string) (instance.State, error) {
	ctx := context.Background()

	// TODO: Allow to specify a termination timeout
	err := provider.Client.ContainerStop(ctx, name, nil)

	if err != nil {
		return instance.ErrorInstanceState(name, err, provider.desiredReplicas)
	}

	return instance.State{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: provider.desiredReplicas,
		Status:          instance.NotReady,
	}, nil
}

func (provider *DockerClassicProvider) GetState(name string) (instance.State, error) {
	ctx := context.Background()

	spec, err := provider.Client.ContainerInspect(ctx, name)

	if err != nil {
		return instance.ErrorInstanceState(name, err, provider.desiredReplicas)
	}

	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "restarting", "removing":
		return instance.NotReadyInstanceState(name, 0, provider.desiredReplicas)
	case "running":
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			if spec.State.Health.Status == "healthy" {
				return instance.ReadyInstanceState(name, provider.desiredReplicas)
			} else if spec.State.Health.Status == "unhealthy" {
				if len(spec.State.Health.Log) >= 1 {
					lastLog := spec.State.Health.Log[len(spec.State.Health.Log)-1]
					return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container is unhealthy: %s (%d)", lastLog.Output, lastLog.ExitCode), provider.desiredReplicas)
				} else {
					return instance.UnrecoverableInstanceState(name, "container is unhealthy: no log available", provider.desiredReplicas)
				}
			} else {
				return instance.NotReadyInstanceState(name, 0, provider.desiredReplicas)
			}
		}
		return instance.ReadyInstanceState(name, provider.desiredReplicas)
	case "exited":
		if spec.State.ExitCode != 0 {
			return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode), provider.desiredReplicas)
		}
		return instance.NotReadyInstanceState(name, 0, provider.desiredReplicas)
	case "dead":
		return instance.UnrecoverableInstanceState(name, "container in \"dead\" state cannot be restarted", provider.desiredReplicas)
	default:
		return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status), provider.desiredReplicas)
	}
}
