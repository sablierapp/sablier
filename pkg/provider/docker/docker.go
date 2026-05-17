package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/client"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client   client.APIClient
	l        *slog.Logger
	strategy string
	tracer   trace.Tracer
}

func New(ctx context.Context, cli *client.Client, logger *slog.Logger, strategy string) (*Provider, error) {
	logger = logger.With(slog.String("provider", "docker"), slog.String("strategy", strategy))

	serverVersion, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %v", err)
	}

	logger.InfoContext(ctx, "connection established with docker",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
	)
	return &Provider{
		Client:   cli,
		l:        logger,
		strategy: strategy,
		tracer:   otel.Tracer("github.com/sablierapp/sablier/pkg/provider/docker"),
	}, nil
}
