package nomad

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
	ctx, span := p.tracer.Start(ctx, "nomad.instance.stop",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	job, tg, err := p.getGroup(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot stop instance %q: %w", name, err)
	}
	jobID, group := deref(job.ID), deref(tg.Name)
	span.SetAttributes(attribute.String("nomad.job", jobID), attribute.String("nomad.group", group))

	target := max(sablier.ScaleConfigFromLabels(groupMeta(job, tg)).Idle.Replicas, 0)
	span.SetAttributes(attribute.Int("replicas", int(target)))

	if deref(job.Stop) {
		span.SetAttributes(attribute.String("operation", "noop.job_stopped"))
		p.l.DebugContext(ctx, "job is stopped, nothing to scale", slog.String("name", name))
		return nil
	}

	if int32(deref(tg.Count)) == target {
		span.SetAttributes(attribute.String("operation", "noop.already_stopped"))
		p.l.DebugContext(ctx, "task group already at the idle count", slog.String("name", name), slog.Int("replicas", int(target)))
		return nil
	}

	span.SetAttributes(attribute.String("operation", "scale"))
	if target > 0 {
		p.l.InfoContext(ctx, "keeping task group running with idle replicas", slog.String("name", name), slog.Int("replicas", int(target)))
	} else {
		p.l.DebugContext(ctx, "scaling task group to zero", slog.String("name", name))
	}
	return p.scale(ctx, job, tg, target, "scaled down by Sablier")
}
