package kubernetes

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "kubernetes.instance.start",
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

	// CloudNativePG Clusters are resumed by clearing the hibernation annotation
	// rather than scaling a replica count, so they bypass the scale-mode logic.
	if parsed.Kind == KindCNPGCluster {
		span.SetAttributes(attribute.String("operation", "cnpg_resume"))
		return p.clusterHibernate(ctx, parsed, false)
	}

	labels, err := p.getWorkloadLabels(ctx, parsed)
	if err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(labels)
	if sc.Idle.Replicas >= 1 || sc.Active.Replicas > 1 || sc.Active.CPU != "" || sc.Active.Memory != "" {
		span.SetAttributes(
			attribute.String("operation", "scale_mode"),
			attribute.Int("replicas", int(sc.Active.Replicas)),
			attribute.String("cpu", sc.Active.CPU),
			attribute.String("memory", sc.Active.Memory),
		)
		p.l.DebugContext(ctx, "applying active resources (scale mode)",
			slog.String("name", name),
			slog.Int("replicas", int(sc.Active.Replicas)),
			slog.String("cpu", sc.Active.CPU),
			slog.String("memory", sc.Active.Memory),
		)
		if err := p.scale(ctx, parsed, sc.Active.Replicas); err != nil {
			return err
		}
		if sc.Active.CPU != "" || sc.Active.Memory != "" {
			return p.scaleResources(ctx, parsed, sc.Active.CPU, sc.Active.Memory)
		}
		return nil
	}

	span.SetAttributes(
		attribute.String("operation", "scale"),
		attribute.Int("replicas", int(sc.Active.Replicas)),
	)
	return p.scale(ctx, parsed, sc.Active.Replicas)
}
