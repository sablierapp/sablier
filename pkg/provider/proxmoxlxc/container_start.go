package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceDependencies(_ context.Context, _ string) ([]sablier.InstanceDependency, error) {
	return nil, nil
}

func (p *Provider) InstanceStart(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "proxmoxlxc.instance.start",
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

	if ct.Status == "running" {
		span.SetAttributes(attribute.String("operation", "noop.already_running"))
		p.l.DebugContext(ctx, "container already running", slog.String("name", ref.name), slog.Int("vmid", ref.vmid))
		return nil
	}

	span.SetAttributes(attribute.String("operation", "start"))
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
