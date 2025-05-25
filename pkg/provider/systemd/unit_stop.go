package systemd

import (
	"context"
	"fmt"
	"log/slog"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping systemd unit", slog.String("name", name))

	ch := make(chan string, 1)
	defer close(ch)

	_, err := p.Con.StopUnitContext(ctx, name, "replace", ch)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop systemd unit", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop systemd unit %s: %w", name, err)
	}
	select {
	case <-ctx.Done():
		p.l.ErrorContext(ctx, "context cancelled while waiting for systemd unit to stop", slog.String("name", name))
		return ctx.Err()
	case status := <-ch:
		if status != "done" {
			p.l.DebugContext(ctx, "systemd unit stopped", slog.String("name", name))
			return nil
		}
		p.l.ErrorContext(ctx, "systemd unit failed to stop", slog.String("name", name), slog.String("status", status))
		return fmt.Errorf("systemd unit %s failed to stop: %s", name, status)
	}

}
