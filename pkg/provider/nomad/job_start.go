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

func (p *Provider) InstanceStart(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "nomad.instance.start",
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
		return fmt.Errorf("cannot start instance %q: %w", name, err)
	}
	jobID, group := deref(job.ID), deref(tg.Name)
	span.SetAttributes(attribute.String("nomad.job", jobID), attribute.String("nomad.group", group))

	target := max(sablier.ScaleConfigFromLabels(groupMeta(job, tg)).Active.Replicas, 1)
	span.SetAttributes(attribute.Int("replicas", int(target)))

	if deref(job.Stop) {
		// A job stopped with "nomad job stop" cannot be scaled. Register it
		// again, the way "nomad job start" does.
		span.SetAttributes(attribute.String("operation", "register"))
		p.l.DebugContext(ctx, "registering stopped job again", slog.String("name", name), slog.Int("replicas", int(target)))
		job.Stop = new(false)
		tg.Count = new(int(target))
		if _, _, err := p.Client.Jobs().Register(job, p.writeOptions(ctx)); err != nil {
			return fmt.Errorf("cannot register stopped job %q again: %w", jobID, err)
		}
		return nil
	}

	if int32(deref(tg.Count)) == target {
		span.SetAttributes(attribute.String("operation", "noop.already_running"))
		p.l.DebugContext(ctx, "task group already at the desired count", slog.String("name", name), slog.Int("replicas", int(target)))
		return nil
	}

	span.SetAttributes(attribute.String("operation", "scale"))
	p.l.DebugContext(ctx, "scaling task group up", slog.String("name", name), slog.Int("replicas", int(target)))
	return p.scale(ctx, jobID, group, target, "scaled up by Sablier")
}

// scale sets the count of a task group through the Nomad scaling API.
func (p *Provider) scale(ctx context.Context, jobID, group string, count int32, message string) error {
	c := int(count)
	if _, _, err := p.Client.Jobs().Scale(jobID, group, &c, message, false, nil, p.writeOptions(ctx)); err != nil {
		return fmt.Errorf("cannot scale task group %q of job %q to %d: %w", group, jobID, count, err)
	}
	return nil
}
