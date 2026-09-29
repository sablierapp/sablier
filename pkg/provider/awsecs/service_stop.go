package awsecs

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "ecs.instance.stop",
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
		attribute.Int("replicas", int(sc.Idle.Replicas)),
	)
	p.l.DebugContext(ctx, "stopping service",
		slog.String("name", name),
		slog.Int("replicas", int(sc.Idle.Replicas)),
	)
	return p.updateDesiredCount(ctx, svc, sc.Idle)
}
