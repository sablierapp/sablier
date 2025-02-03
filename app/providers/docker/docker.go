package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/providers"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/app/instance"
)

// Interface guard
var _ providers.Provider = (*DockerClassicProvider)(nil)

type DockerClassicProvider struct {
	Client          client.APIClient
	desiredReplicas int32
	l               *slog.Logger
}

func NewDockerClassicProvider(ctx context.Context, logger *slog.Logger) (*DockerClassicProvider, error) {
	logger = logger.With(slog.String("provider", "docker"))
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("cannot create docker client: %w", err)
	}

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %w", err)
	}

	logger.InfoContext(ctx, "connection established with docker",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
	)
	return &DockerClassicProvider{
		Client:          cli,
		desiredReplicas: 1,
		l:               logger,
	}, nil
}

func (p *DockerClassicProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	groups := make(map[string][]string)
	for _, c := range containers {
		groupName := c.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}
		group := groups[groupName]
		group = append(group, strings.TrimPrefix(c.Names[0], "/"))
		groups[groupName] = group
	}

	return groups, nil
}

func (p *DockerClassicProvider) Start(ctx context.Context, name string) error {
	return p.Client.ContainerStart(ctx, name, container.StartOptions{})
}

func (p *DockerClassicProvider) Stop(ctx context.Context, name string) error {
	return p.Client.ContainerStop(ctx, name, container.StopOptions{})
}

func (p *DockerClassicProvider) GetState(ctx context.Context, name string) (instance.State, error) {
	spec, err := p.Client.ContainerInspect(ctx, name)
	if err != nil {
		return instance.State{}, fmt.Errorf("cannot inspect container: %w", err)
	}

	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "restarting", "removing":
		return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "running":
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			switch spec.State.Health.Status {
			case "healthy":
				return instance.ReadyInstanceState(name, p.desiredReplicas), nil
			case "unhealthy":
				if len(spec.State.Health.Log) >= 1 {
					lastLog := spec.State.Health.Log[len(spec.State.Health.Log)-1]
					return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container is unhealthy: %s (%d)", lastLog.Output, lastLog.ExitCode), p.desiredReplicas), nil
				}
				return instance.UnrecoverableInstanceState(name, "container is unhealthy: no log available", p.desiredReplicas), nil
			default:
				return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
			}
		}
		return instance.ReadyInstanceState(name, p.desiredReplicas), nil
	case "exited":
		if spec.State.ExitCode != 0 {
			return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode), p.desiredReplicas), nil
		}
		return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "dead":
		return instance.UnrecoverableInstanceState(name, "container in \"dead\" state cannot be restarted", p.desiredReplicas), nil
	default:
		return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status), p.desiredReplicas), nil
	}
}

func (p *DockerClassicProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	msgs, errs := p.Client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "local"),
			filters.Arg("type", string(events.ContainerEventType)),
			filters.Arg("event", "die"),
		),
	})
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				return
			}
			// Send the container that has died to the channel
			instance <- strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		case err, ok := <-errs:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				return
			}
			if errors.Is(err, io.EOF) {
				p.l.ErrorContext(ctx, "event stream closed")
				return
			}
			p.l.ErrorContext(ctx, "event stream error", slog.Any("error", err))
		case <-ctx.Done():
			return
		}
	}
}
