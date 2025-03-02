package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"log/slog"
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
