package dockerswarm

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "swarm.instance.stop",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return fmt.Errorf("cannot get service: %w", err)
	}

	sc := sablier.ScaleConfigFromLabels(service.Spec.Labels)
	span.SetAttributes(
		attribute.String("operation", "scale"),
		attribute.Int("replicas", int(sc.Idle.Replicas)),
		attribute.String("cpu", sc.Idle.CPU),
		attribute.String("memory", sc.Idle.Memory),
	)
	p.l.DebugContext(ctx, "stopping service",
		slog.String("name", name),
		slog.Int("replicas", int(sc.Idle.Replicas)),
		slog.String("cpu", sc.Idle.CPU),
		slog.String("memory", sc.Idle.Memory),
	)
	return p.ServiceUpdateScale(ctx, name, uint64(sc.Idle.Replicas), sc.Idle.CPU, sc.Idle.Memory)
}
