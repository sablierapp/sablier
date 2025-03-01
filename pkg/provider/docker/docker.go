package docker

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/pkg/provider"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Interface guard
var _ provider.Provider = (*DockerClassicProvider)(nil)

type DockerClassicProvider struct {
	Client          client.APIClient
	desiredReplicas int32
	l               *slog.Logger
}

func NewDockerClassicProvider(ctx context.Context, cli *client.Client, logger *slog.Logger) (*DockerClassicProvider, error) {
	logger = logger.With(slog.String("provider", "docker"))

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %v", err)
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
		return nil, err
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
