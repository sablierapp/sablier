package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "docker.instance.stop",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("cannot inspect container: %w", err)
	}

	sc := sablier.ScaleConfigFromLabels(spec.Container.Config.Labels)
	if sc.Idle.Replicas >= 1 {
		if sc.Idle.CPU != "" || sc.Idle.Memory != "" {
			span.SetAttributes(
				attribute.String("operation", "scale_mode.apply_resources"),
				attribute.String("cpu", sc.Idle.CPU),
				attribute.String("memory", sc.Idle.Memory),
			)
			p.l.DebugContext(ctx, "applying idle resources (scale mode)",
				slog.String("name", name),
				slog.String("cpu", sc.Idle.CPU),
				slog.String("memory", sc.Idle.Memory),
			)
			return p.applyResources(ctx, name, sc.Idle.CPU, sc.Idle.Memory)
		}
		span.SetAttributes(
			attribute.String("operation", "scale_mode.noop"),
			attribute.Int("idle_replicas", int(sc.Idle.Replicas)),
		)
		return nil
	}

	if p.strategy == "pause" {
		span.SetAttributes(attribute.String("operation", "pause"))
		return p.dockerPause(ctx, name)
	}
	span.SetAttributes(attribute.String("operation", "stop"))
	return p.dockerStop(ctx, name)
}

func (p *Provider) dockerStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping container", slog.String("name", name))
	_, err := p.Client.ContainerStop(ctx, name, client.ContainerStopOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "waiting for container to stop", slog.String("name", name))
	result := p.Client.ContainerWait(ctx, name, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case response := <-result.Result:
		p.l.DebugContext(ctx, "container stopped", slog.String("name", name), slog.Int64("exit_code", response.StatusCode))
		return nil
	case err := <-result.Error:
		p.l.ErrorContext(ctx, "cannot wait for container to stop", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot wait for container %s to stop: %w", name, err)
	case <-ctx.Done():
		p.l.ErrorContext(ctx, "context cancelled while waiting for container to stop", slog.String("name", name))
		return ctx.Err()
	}
}

func (p *Provider) dockerPause(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "pausing container", slog.String("name", name))
	_, err := p.Client.ContainerPause(ctx, name, client.ContainerPauseOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot pause container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot pause container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "container paused", slog.String("name", name))
	return nil
}
