package podman

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/containers/podman/v5/pkg/bindings/containers"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "starting container", "name", name)

	// TODO: Create a context from the ctx argument with the p.conn
	err := containers.Start(p.conn, name, nil)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}
