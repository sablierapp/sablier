package podman

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/pkg/sablier"
	"log/slog"

	"github.com/containers/podman/v5/pkg/bindings/system"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	conn            context.Context
	desiredReplicas int32
	l               *slog.Logger
}

func New(ctx context.Context, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "podman"))

	version, err := system.Version(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get podman version: %v", err)
	}
	logger.InfoContext(ctx, "connection established with podman",
		slog.String("version", version.Server.Version),
		slog.String("api_version", version.Server.APIVersion),
	)
	return &Provider{
		conn:            ctx,
		desiredReplicas: 1,
		l:               logger,
	}, nil
}
