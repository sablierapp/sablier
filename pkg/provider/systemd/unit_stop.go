package systemd

import (
	"context"
	"fmt"
	"log/slog"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping systemd unit", slog.String("name", name))

	sc, err := p.scaleConfig(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot read scale config for systemd unit %q: %w", name, err)
	}

	if sc.Idle.Replicas >= 1 {
		if sc.Idle.Replicas > 1 {
			p.l.WarnContext(ctx, "idle replicas are capped at 1 for systemd units", slog.String("name", name), slog.Int("replicas", int(sc.Idle.Replicas)))
		}
		p.l.InfoContext(ctx, "keeping systemd unit running with idle resources", slog.String("name", name))
		if err := p.applyResources(ctx, name, sc.Idle, sc.Active); err != nil {
			return fmt.Errorf("cannot apply idle resources to systemd unit %q: %w", name, err)
		}
		return nil
	}
	return p.stopUnit(ctx, name)
}

func (p *Provider) stopUnit(ctx context.Context, name string) error {
	ch := make(chan string, 1)

	_, err := p.Con.StopUnitContext(ctx, name, "replace", ch)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop systemd unit", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop systemd unit %q: %w", name, err)
	}
	if err := p.waitUnitJob(ctx, name, "stop", ch); err != nil {
		return err
	}
	p.l.DebugContext(ctx, "systemd unit stopped", slog.String("name", name))
	return nil
}
