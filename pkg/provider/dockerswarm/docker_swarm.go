package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"github.com/sablierapp/sablier/pkg/sablier"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client          client.APIClient
	desiredReplicas int32

	l *slog.Logger
}

func New(ctx context.Context, cli *client.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "swarm"))

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %w", err)
	}

	logger.InfoContext(ctx, "connection established with docker swarm",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
	)

	return &Provider{
		Client:          cli,
		desiredReplicas: 1,
		l:               logger,
	}, nil

}

func (p *Provider) ServiceUpdateReplicas(ctx context.Context, name string, replicas uint64) error {
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

func (p *Provider) getInstanceName(name string, service swarm.Service) string {
	if name == service.Spec.Name {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, service.Spec.Name)
}
