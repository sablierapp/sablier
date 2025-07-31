package podman

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/containers/podman/v5/pkg/bindings/containers"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {

	p.l.DebugContext(ctx, "stopping container", slog.String("name", name))
	err := containers.Stop(p.conn, name, nil)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "waiting for container to stop", slog.String("name", name))
	code, err := containers.Wait(p.conn, name, &containers.WaitOptions{
		Conditions: []string{"stopped"},
	})
	if err != nil {
		return fmt.Errorf("cannot wait for container %s to stop: %w", name, err)
	}
	p.l.DebugContext(ctx, "container stopped", slog.String("name", name), slog.Int("exit_code", int(code)))

	return nil
}
