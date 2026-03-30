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

	// Clear any previous start failure so a retry is possible.
	p.mu.Lock()
	delete(p.failedStarts, name)
	p.mu.Unlock()

	p.l.DebugContext(ctx, "starting container", slog.String("name", ref.name), slog.Int("vmid", ref.vmid), slog.String("node", ref.node))

	task, err := ct.Start(ctx)
	if err != nil {
		return fmt.Errorf("cannot start container %q (VMID %d): %w", ref.name, ref.vmid, err)
	}

	if err := task.Wait(ctx, 1*time.Second, 60*time.Second); err != nil {
		// Timeout or API error — record as failed start so InstanceInspect
		// can report UnrecoverableInstanceState instead of retrying indefinitely.
		msg := fmt.Sprintf("start task failed for container %q (VMID %d): %v", ref.name, ref.vmid, err)
		p.l.WarnContext(ctx, msg)
		p.mu.Lock()
		p.failedStarts[name] = msg
		p.mu.Unlock()
		return nil
	}

	// task.Wait() returns nil when the task completes, but does not check ExitStatus.
	// We must inspect task.IsFailed to detect e.g. "startup for container '100' failed".
	if task.IsFailed {
		msg := fmt.Sprintf("start task failed for container %q (VMID %d): %s", ref.name, ref.vmid, task.ExitStatus)
		p.l.WarnContext(ctx, msg)
		p.mu.Lock()
		p.failedStarts[name] = msg
		p.mu.Unlock()
		return nil
	}

	p.l.DebugContext(ctx, "container started", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
	return nil
}
