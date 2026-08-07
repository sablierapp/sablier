package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "proxmoxlxc.instance.stop",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	ref, err := p.resolve(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot resolve instance %q: %w", name, err)
	}

	span.SetAttributes(
		attribute.String("proxmox.node", ref.node),
		attribute.Int("proxmox.vmid", ref.vmid),
	)

	ct, err := p.getContainer(ctx, ref)
	if err != nil {
		return err
	}

	// Clear any pending start task — the stop supersedes it.
	p.mu.Lock()
	delete(p.pendingTasks, name)
	p.mu.Unlock()

	if ct.Status == "stopped" {
		span.SetAttributes(attribute.String("operation", "noop.already_stopped"))
		p.l.DebugContext(ctx, "container already stopped", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
		return nil
	}

	span.SetAttributes(attribute.String("operation", "stop"))
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
