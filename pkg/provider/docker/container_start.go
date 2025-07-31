package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	// TODO: InstanceStart should block until the container is ready.
	p.l.DebugContext(ctx, "starting container", "name", name)
	err := p.Client.ContainerStart(ctx, name, container.StartOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}
