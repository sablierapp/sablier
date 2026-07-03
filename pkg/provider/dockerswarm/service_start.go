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

func (p *Provider) InstanceDependencies(_ context.Context, _ string) ([]sablier.InstanceDependency, error) {
	return nil, nil
}

func (p *Provider) InstanceStart(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "swarm.instance.start",
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
		attribute.Int("replicas", int(sc.Active.Replicas)),
		attribute.String("cpu", sc.Active.CPU),
		attribute.String("memory", sc.Active.Memory),
	)
	p.l.DebugContext(ctx, "starting service",
		slog.String("name", name),
		slog.Int("replicas", int(sc.Active.Replicas)),
		slog.String("cpu", sc.Active.CPU),
		slog.String("memory", sc.Active.Memory),
	)
	return p.ServiceUpdateScale(ctx, name, uint64(sc.Active.Replicas), sc.Active.CPU, sc.Active.Memory)
}
