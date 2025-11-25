package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping container", slog.String("name", name))
	_, err := p.Client.ContainerStop(ctx, name, client.ContainerStopOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "waiting for container to stop", slog.String("name", name))
	result := p.Client.ContainerWait(ctx, name, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case response := <-result.Result:
		p.l.DebugContext(ctx, "container stopped", slog.String("name", name), slog.Int64("exit_code", response.StatusCode))
		return nil
	case err := <-result.Error:
		p.l.ErrorContext(ctx, "cannot wait for container to stop", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot wait for container %s to stop: %w", name, err)
	case <-ctx.Done():
		p.l.ErrorContext(ctx, "context cancelled while waiting for container to stop", slog.String("name", name))
		return ctx.Err()
	}
}
