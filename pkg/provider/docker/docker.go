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
	Client client.APIClient
	l      *slog.Logger
	// apiVersion is the negotiated Docker daemon API version (e.g. "1.51"),
	// captured at construction time. It is used to warn when a requested
	// feature is not supported by the connected daemon.
	apiVersion string
	strategy   string
	tracer     trace.Tracer

	// HonorRestartPolicy makes InstanceInspect honor the container's restart
	// policy when it exits successfully (exit code 0): "no"/"on-failure" are
	// reported as completed (a one-shot/init container that finished its job),
	// while an exited "always"/"unless-stopped" container was stopped and is
	// reported as stopped. When false, a successfully exited container is always
	// reported as stopped (historical behavior).
	//
	// Deprecated: transitional flag for backward compatibility; honoring the
	// restart policy will become the default in v2.
	HonorRestartPolicy bool
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
		Client:     cli,
		l:          logger,
		apiVersion: serverVersion.APIVersion,
		strategy:   strategy,
		tracer:     otel.Tracer("github.com/sablierapp/sablier/pkg/provider/docker"),
	}, nil
}
