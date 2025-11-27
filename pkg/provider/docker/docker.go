package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client          client.APIClient
	desiredReplicas int32
	l               *slog.Logger
	strategy        string
}

func New(ctx context.Context, cli *client.Client, logger *slog.Logger, strategy string) (*Provider, error) {
	logger = logger.With(slog.String("provider", "docker"), slog.String("strategy", strategy))

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
		strategy:        strategy,
	}, nil
}
