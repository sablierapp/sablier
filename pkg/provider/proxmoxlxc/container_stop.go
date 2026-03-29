package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	ref, err := p.resolve(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot resolve instance %q: %w", name, err)
	}

	ct, err := p.getContainer(ctx, ref)
	if err != nil {
		return err
	}

	if ct.Status == "stopped" {
		p.l.DebugContext(ctx, "container already stopped", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
		return nil
	}

	p.l.DebugContext(ctx, "stopping container", slog.String("name", ref.name), slog.Int("vmid", ref.vmid), slog.String("node", ref.node))

	task, err := ct.Stop(ctx)
	if err != nil {
		return fmt.Errorf("cannot stop container %q (VMID %d): %w", ref.name, ref.vmid, err)
	}

	if err := task.Wait(ctx, 1*time.Second, taskTimeout(ctx)); err != nil {
		return fmt.Errorf("cannot wait for container %q (VMID %d) to stop: %w", ref.name, ref.vmid, err)
	}

	p.l.DebugContext(ctx, "container stopped", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
	return nil
}
