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

	// Resolve and start depends_on dependencies in order. A cyclic tree is
	// invalid and ignored: the instance is started on its own instead.
	// See https://github.com/sablierapp/sablier/issues/792
	tree, err := p.buildDependencyTree(ctx, name)
	if err != nil {
		return err
	}

	if cyclic, path := tree.hasCycle(); cyclic {
		p.l.WarnContext(ctx, "dependency tree ignored, cycle detected",
			slog.String("instance", name),
			slog.String("cycle", path),
		)
		return p.startSingle(ctx, tree.root)
	}

	return p.startTree(ctx, tree)
}

// startSingle starts a single container, applying the configured scale mode or
// strategy. It does not resolve depends_on dependencies.
func (p *Provider) startSingle(ctx context.Context, name string) (err error) {
	span := trace.SpanFromContext(ctx)

	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("cannot inspect container: %w", err)
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
