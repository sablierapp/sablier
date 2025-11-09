package podman

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	spec, err := containers.Inspect(p.conn, name, nil)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot inspect container after starting", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot inspect container %s after starting: %w", name, err)
	}

	status, err := define.StringToContainerStatus(spec.State.Status)
	if err != nil {
		return fmt.Errorf("cannot convert container status: %w", err)
	}

	if status == define.ContainerStateRunning {

	}

	// TODO: Create a context from the ctx argument with the p.conn
	err := containers.Start(p.conn, name, nil)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}

	return nil
}
