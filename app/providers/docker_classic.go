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
	Client client.ContainerAPIClient
}

func NewDockerClassicProvider() (*DockerClassicProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
		return nil, err
	}
	return &DockerClassicProvider{
		Client: cli,
	}, nil
}

func (provider *DockerClassicProvider) Start(name string) (instance.State, error) {
	ctx := context.Background()

	err := provider.Client.ContainerStart(ctx, name, types.ContainerStartOptions{})

	if err != nil {
		return instance.ErrorInstanceState(name, err)
	}

	return instance.State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          instance.NotReady,
	}, err
}

func (provider *DockerClassicProvider) Stop(name string) (instance.State, error) {
	ctx := context.Background()

	// TODO: Allow to specify a termination timeout
	err := provider.Client.ContainerStop(ctx, name, nil)

	if err != nil {
		return instance.ErrorInstanceState(name, err)
	}

	return instance.State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          instance.NotReady,
	}, nil
}

func (provider *DockerClassicProvider) GetState(name string) (instance.State, error) {
	ctx := context.Background()

	spec, err := provider.Client.ContainerInspect(ctx, name)

	if err != nil {
		return instance.ErrorInstanceState(name, err)
	}

	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "restarting", "removing":
		return instance.NotReadyInstanceState(name)
	case "running":
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			if spec.State.Health.Status == "healthy" {
				return instance.ReadyInstanceState(name)
			} else if spec.State.Health.Status == "unhealthy" {
				if len(spec.State.Health.Log) >= 1 {
					lastLog := spec.State.Health.Log[len(spec.State.Health.Log)-1]
					return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container is unhealthy: %s (%d)", lastLog.Output, lastLog.ExitCode))
				} else {
					return instance.UnrecoverableInstanceState(name, "container is unhealthy: no log available")
				}
			} else {
				return instance.NotReadyInstanceState(name)
			}
		}
		return instance.ReadyInstanceState(name)
	case "exited":
		if spec.State.ExitCode != 0 {
			return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode))
		}
		return instance.NotReadyInstanceState(name)
	case "dead":
		return instance.UnrecoverableInstanceState(name, "container in \"dead\" state cannot be restarted")
	default:
		return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status))
	}
}
