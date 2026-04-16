package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"
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

	// Store the task so InstanceInspect can track its progress via Ping().
	// This mirrors the Docker provider pattern: InstanceStart returns quickly,
	// and readiness is determined by polling InstanceInspect.
	p.mu.Lock()
	p.pendingTasks[name] = &pendingTask{task: task}
	p.mu.Unlock()

	p.l.DebugContext(ctx, "start task submitted",
		slog.String("name", ref.name),
		slog.Int("vmid", ref.vmid),
		slog.String("upid", string(task.UPID)),
	)
	return nil
}
