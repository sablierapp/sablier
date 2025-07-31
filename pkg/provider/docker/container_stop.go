package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping container", slog.String("name", name))
	err := p.Client.ContainerStop(ctx, name, container.StopOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "waiting for container to stop", slog.String("name", name))
	waitC, errC := p.Client.ContainerWait(ctx, name, container.WaitConditionNotRunning)
	select {
	case response := <-waitC:
		p.l.DebugContext(ctx, "container stopped", slog.String("name", name), slog.Int64("exit_code", response.StatusCode))
		return nil
	case err := <-errC:
		p.l.ErrorContext(ctx, "cannot wait for container to stop", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot wait for container %s to stop: %w", name, err)
	case <-ctx.Done():
		p.l.ErrorContext(ctx, "context cancelled while waiting for container to stop", slog.String("name", name))
		return ctx.Err()
	}
}
