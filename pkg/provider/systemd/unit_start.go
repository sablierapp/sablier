package systemd

import (
	"context"
	"fmt"
	"log/slog"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "starting systemd unit", slog.String("name", name))

	sc, err := p.scaleConfig(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot read scale config for systemd unit %q: %w", name, err)
	}

	if sc.Active.Replicas > 1 {
		p.l.WarnContext(ctx, "active replicas are capped at 1 for systemd units", slog.String("name", name), slog.Int("replicas", int(sc.Active.Replicas)))
	}
	if sc.Active.HasResources() || sc.Idle.HasResources() {
		if err := p.applyResources(ctx, name, sc.Active, sc.Idle); err != nil {
			return fmt.Errorf("cannot apply active resource profile to systemd unit %q: %w", name, err)
		}
	}
	return p.startUnit(ctx, name)
}

func (p *Provider) startUnit(ctx context.Context, name string) error {
	ch := make(chan string, 1)

	_, err := p.Con.StartUnitContext(ctx, name, "replace", ch)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start systemd unit", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start systemd unit %q: %w", name, err)
	}
	if err := p.waitUnitJob(ctx, name, "start", ch); err != nil {
		return err
	}
	p.l.DebugContext(ctx, "systemd unit started", slog.String("name", name))
	return nil
}

func (p *Provider) waitUnitJob(ctx context.Context, name, action string, ch <-chan string) error {
	select {
	case <-ctx.Done():
		p.l.ErrorContext(ctx, "context cancelled while waiting for systemd unit to "+action, slog.String("name", name))
		return ctx.Err()
	case status := <-ch:
		if status == "done" {
			return nil
		}
		p.l.ErrorContext(ctx, "systemd unit failed to "+action, slog.String("name", name), slog.String("status", status))
		return fmt.Errorf("systemd unit %q failed to %s: %s", name, action, status)
	}
}
