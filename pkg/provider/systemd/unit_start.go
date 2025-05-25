package systemd

import (
	"context"
	"fmt"
	"log/slog"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "starting systemd unit", slog.String("name", name))

	ch := make(chan string, 1)
	defer close(ch)

	_, err := p.Con.StartUnitContext(ctx, name, "replace", ch)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start systemd unit", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start systemd unit %s: %w", name, err)
	}
	select {
	case <-ctx.Done():
		p.l.ErrorContext(ctx, "context cancelled while waiting for systemd unit to start", slog.String("name", name))
		return ctx.Err()
	case status := <-ch:
		if status != "done" {
			p.l.DebugContext(ctx, "systemd unit started", slog.String("name", name))
			return nil
		}
		p.l.ErrorContext(ctx, "systemd unit failed to start", slog.String("name", name), slog.String("status", status))
		return fmt.Errorf("systemd unit %s failed to start: %s", name, status)
	}

}
