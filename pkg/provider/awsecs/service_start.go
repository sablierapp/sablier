package awsecs

import (
	"context"
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
	ctx, span := p.tracer.Start(ctx, "ecs.instance.start",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	svc, err := p.describeService(ctx, name)
	if err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(tagsToLabels(svc.Tags))
	span.SetAttributes(
		attribute.String("operation", "scale"),
		attribute.Int("replicas", int(sc.Active.Replicas)),
	)
	p.l.DebugContext(ctx, "starting service",
		slog.String("name", name),
		slog.Int("replicas", int(sc.Active.Replicas)),
	)
	return p.updateDesiredCount(ctx, svc, sc.Active)
}
