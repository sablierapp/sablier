package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/pkg/sablier"
	"log/slog"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client          client.APIClient
	desiredReplicas int32
	l               *slog.Logger
}

func New(ctx context.Context, cli *client.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "docker"))

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %v", err)
	}

	logger.InfoContext(ctx, "connection established with docker",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
	)
	return &Provider{
		Client:          cli,
		desiredReplicas: 1,
		l:               logger,
	}, nil
}
