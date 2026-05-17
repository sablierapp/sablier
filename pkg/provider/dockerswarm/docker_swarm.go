package dockerswarm

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
	Client client.APIClient

	l      *slog.Logger
	tracer trace.Tracer
}

func New(ctx context.Context, cli *client.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "swarm"))

	serverVersion, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %w", err)
	}

	logger.InfoContext(ctx, "connection established with docker swarm",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
	)

	return &Provider{
		Client: cli,
		l:      logger,
		tracer: otel.Tracer("github.com/sablierapp/sablier/pkg/provider/dockerswarm"),
	}, nil

}
