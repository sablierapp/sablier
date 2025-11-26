package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	if p.strategy == "pause" {
		return p.dockerUnpause(ctx, name)
	}
	return p.dockerStart(ctx, name)
}

func (p *Provider) dockerStart(ctx context.Context, name string) error {
	// TODO: InstanceStart should block until the container is ready.
	p.l.DebugContext(ctx, "starting container", "name", name)
	err := p.Client.ContainerStart(ctx, name, container.StartOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}

func (p *Provider) dockerUnpause(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "unpausing container", slog.String("name", name))
	err := p.Client.ContainerUnpause(ctx, name)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot unpause container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot unpause container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "container unpaused", slog.String("name", name))
	return nil
}
