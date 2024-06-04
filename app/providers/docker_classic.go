package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type DockerClassicProvider struct {
	Client          client.APIClient
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

func (provider *DockerClassicProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("%s=true", enableLabel))

	containers, err := provider.Client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, container := range containers {
		groupName := container.Labels[groupLabel]
		if len(groupName) == 0 {
			groupName = defaultGroupValue
		}
		group := groups[groupName]
		group = append(group, strings.TrimPrefix(container.Names[0], "/"))
		groups[groupName] = group
	}

	log.Debug(fmt.Sprintf("%v", groups))

	return groups, nil
}

func (provider *DockerClassicProvider) Start(ctx context.Context, name string) (instance.State, error) {
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

func (provider *DockerClassicProvider) Stop(ctx context.Context, name string) (instance.State, error) {
	err := provider.Client.ContainerStop(ctx, name, container.StopOptions{})

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

func (provider *DockerClassicProvider) GetState(ctx context.Context, name string) (instance.State, error) {
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

func (provider *DockerClassicProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	msgs, errs := provider.Client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "local"),
			filters.Arg("type", string(events.ContainerEventType)),
			filters.Arg("event", "die"),
		),
	})
	for {
		select {
		case msg := <-msgs:
			// Send the container that has died to the channel
			instance <- strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		case err := <-errs:
			if errors.Is(err, io.EOF) {
				log.Debug("provider event stream closed")
				return
			}
            		if err != nil {
                		log.Error("error while processing Docker events:", err)
            		}			
		case <-ctx.Done():
			return
        	default:
            		// Add a small delay to prevent high CPU usage
            		time.Sleep(100 * time.Millisecond)
		}
	}
}
