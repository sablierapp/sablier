// Package podman provides a Sablier provider for Podman by wrapping the Docker provider.
// Podman exposes a Docker-compatible API, so we simply connect a Docker client to the
// Podman socket instead of using the heavy podman bindings.
package podman

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

// Provider wraps the Docker provider, connecting to a Podman socket that exposes
// the Docker-compatible REST API.
type Provider struct {
	*docker.Provider
}

// New creates a Podman provider using the given Docker API client (pointing at a Podman
// socket) and delegates all operations to the Docker provider.
func New(ctx context.Context, cli *client.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "podman"))

	inner, err := docker.New(ctx, cli, logger, "stop")
	if err != nil {
		return nil, fmt.Errorf("cannot connect to podman: %w", err)
	}

	return &Provider{Provider: inner}, nil
}
