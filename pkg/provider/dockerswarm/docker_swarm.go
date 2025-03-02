package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/pkg/provider"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// Interface guard
var _ provider.Provider = (*DockerSwarmProvider)(nil)

type DockerSwarmProvider struct {
	Client          client.APIClient
	desiredReplicas int32

	l *slog.Logger
}

func NewDockerSwarmProvider(ctx context.Context, cli *client.Client, logger *slog.Logger) (*DockerSwarmProvider, error) {
	logger = logger.With(slog.String("provider", "swarm"))

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %w", err)
	}

	logger.InfoContext(ctx, "connection established with docker swarm",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
	)

	return &DockerSwarmProvider{
		Client:          cli,
		desiredReplicas: 1,
		l:               logger,
	}, nil

}

func (p *DockerSwarmProvider) Start(ctx context.Context, name string) error {
	return p.scale(ctx, name, uint64(p.desiredReplicas))
}

func (p *DockerSwarmProvider) Stop(ctx context.Context, name string) error {
	return p.scale(ctx, name, 0)
}

func (p *DockerSwarmProvider) scale(ctx context.Context, name string, replicas uint64) error {
	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return err
	}

	foundName := p.getInstanceName(name, *service)
	if service.Spec.Mode.Replicated == nil {
		return errors.New("swarm service is not in \"replicated\" mode")
	}

	service.Spec.Mode.Replicated.Replicas = &replicas

	response, err := p.Client.ServiceUpdate(ctx, service.ID, service.Meta.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return err
	}

	if len(response.Warnings) > 0 {
		return fmt.Errorf("warning received updating swarm service [%s]: %s", foundName, strings.Join(response.Warnings, ", "))
	}

	return nil
}

func (p *DockerSwarmProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	services, err := p.Client.ServiceList(ctx, types.ServiceListOptions{
		Filters: f,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, service := range services {
		groupName := service.Spec.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		group = append(group, service.Spec.Name)
		groups[groupName] = group
	}

	return groups, nil
}

func (p *DockerSwarmProvider) getInstanceName(name string, service swarm.Service) string {
	if name == service.Spec.Name {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, service.Spec.Name)
}

func (p *DockerSwarmProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	msgs, errs := p.Client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "swarm"),
			filters.Arg("type", "service"),
		),
	})

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				if msg.Actor.Attributes["replicas.new"] == "0" {
					instance <- msg.Actor.Attributes["name"]
				} else if msg.Action == "remove" {
					instance <- msg.Actor.Attributes["name"]
				}
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
	}()
}
