package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	ref, err := p.resolve(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot resolve instance %q: %w", name, err)
	}

	ct, err := p.getContainer(ctx, ref)
	if err != nil {
		return err
	}

	if ct.Status == "running" {
		p.l.DebugContext(ctx, "container already running", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
		return nil
	}

	p.l.DebugContext(ctx, "starting container", slog.String("name", ref.name), slog.Int("vmid", ref.vmid), slog.String("node", ref.node))

	task, err := ct.Start(ctx)
	if err != nil {
		return fmt.Errorf("cannot start container %q (VMID %d): %w", ref.name, ref.vmid, err)
	}

	if err := task.Wait(ctx, 1*time.Second, 60*time.Second); err != nil {
		return fmt.Errorf("error waiting for container %q to start: %w", ref.name, err)
	}

	p.l.DebugContext(ctx, "container started", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
	return nil
}
