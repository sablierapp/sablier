package kubernetes

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "kubernetes.instance.stop",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	labels, err := p.getWorkloadLabels(ctx, parsed)
	if err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(labels)
	if sc.Idle.Replicas >= 1 {
		span.SetAttributes(
			attribute.String("operation", "scale_mode"),
			attribute.Int("replicas", int(sc.Idle.Replicas)),
			attribute.String("cpu", sc.Idle.CPU),
			attribute.String("memory", sc.Idle.Memory),
		)
		p.l.DebugContext(ctx, "applying idle resources (scale mode)",
			slog.String("name", name),
			slog.Int("replicas", int(sc.Idle.Replicas)),
			slog.String("cpu", sc.Idle.CPU),
			slog.String("memory", sc.Idle.Memory),
		)
		if err := p.scale(ctx, parsed, sc.Idle.Replicas); err != nil {
			return err
		}
		if sc.Idle.CPU != "" || sc.Idle.Memory != "" {
			return p.scaleResources(ctx, parsed, sc.Idle.CPU, sc.Idle.Memory)
		}
		return nil
	}

	span.SetAttributes(
		attribute.String("operation", "scale_to_zero"),
	)
	return p.scale(ctx, parsed, 0)
}
