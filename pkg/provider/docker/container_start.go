package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/client"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "docker.instance.start",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	return p.instanceStart(ctx, name, make(map[string]struct{}))
}

// instanceStart starts a single instance, recursively starting and waiting for
// its Docker Compose depends_on dependencies first. The started set guards
// against starting the same container twice within a single start invocation
// (and against dependency cycles).
func (p *Provider) instanceStart(ctx context.Context, name string, started map[string]struct{}) (err error) {
	if _, ok := started[name]; ok {
		return nil
	}
	started[name] = struct{}{}

	span := trace.SpanFromContext(ctx)

	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("cannot inspect container: %w", err)
	}

	// Resolve and start Docker Compose depends_on dependencies before starting
	// the instance itself so that the ordering and conditions declared in the
	// compose file are respected.
	// See https://github.com/sablierapp/sablier/issues/792
	if err = p.startDependencies(ctx, spec.Container.Config.Labels, started); err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(spec.Container.Config.Labels)
	if sc.Idle.Replicas >= 1 || sc.Active.CPU != "" || sc.Active.Memory != "" {
		if sc.Active.CPU != "" || sc.Active.Memory != "" {
			span.SetAttributes(
				attribute.String("operation", "scale_mode.apply_resources"),
				attribute.String("cpu", sc.Active.CPU),
				attribute.String("memory", sc.Active.Memory),
			)
			p.l.DebugContext(ctx, "applying active resources (scale mode)",
				slog.String("name", name),
				slog.String("cpu", sc.Active.CPU),
				slog.String("memory", sc.Active.Memory),
			)
			return p.applyResources(ctx, name, sc.Active.CPU, sc.Active.Memory)
		}
		span.SetAttributes(
			attribute.String("operation", "scale_mode.noop"),
			attribute.Int("idle_replicas", int(sc.Idle.Replicas)),
		)
		return nil
	}

	if p.strategy == "pause" {
		span.SetAttributes(attribute.String("operation", "unpause"))
		return p.dockerUnpause(ctx, name)
	}
	span.SetAttributes(attribute.String("operation", "start"))
	return p.dockerStart(ctx, name)
}

func (p *Provider) dockerStart(ctx context.Context, name string) error {
	// TODO: InstanceStart should block until the container is ready.
	p.l.DebugContext(ctx, "starting container", slog.String("name", name))
	_, err := p.Client.ContainerStart(ctx, name, client.ContainerStartOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}

func (p *Provider) dockerUnpause(ctx context.Context, name string) error {
	inspected, inspectErr := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if inspectErr != nil {
		p.l.ErrorContext(ctx, "cannot inspect container before unpausing", slog.String("name", name), slog.Any("error", inspectErr))
		return fmt.Errorf("cannot inspect container %s before unpausing: %w", name, inspectErr)
	}

	if !inspected.Container.State.Paused {
		p.l.DebugContext(ctx, "container is not paused, starting container", slog.String("name", name))
		return p.dockerStart(ctx, name)
	}

	p.l.DebugContext(ctx, "unpausing container", slog.String("name", name))
	_, err := p.Client.ContainerUnpause(ctx, name, client.ContainerUnpauseOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot unpause container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot unpause container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "container unpaused", slog.String("name", name))
	return nil
}
