package nomad

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/nomad/api"
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
		return p.register(ctx, job)
	}

	if int32(deref(tg.Count)) == target {
		span.SetAttributes(attribute.String("operation", "noop.already_running"))
		p.l.DebugContext(ctx, "task group already at the desired count", slog.String("name", name), slog.Int("replicas", int(target)))
		return nil
	}

	span.SetAttributes(attribute.String("operation", "scale"))
	p.l.DebugContext(ctx, "scaling task group up", slog.String("name", name), slog.Int("replicas", int(target)))
	return p.scale(ctx, job, tg, target, "scaled up by Sablier")
}

// scale sets the count of a task group through the Nomad scaling API.
func (p *Provider) scale(ctx context.Context, job *api.Job, tg *api.TaskGroup, count int32, message string) error {
	jobID, group := deref(job.ID), deref(tg.Name)
	c := int(count)
	_, _, err := p.Client.Jobs().Scale(jobID, group, &c, message, false, nil, p.writeOptions(ctx))
	if err == nil {
		return nil
	}
	if !isBlockedByDeployment(err) {
		return fmt.Errorf("cannot scale task group %q of job %q to %d: %w", group, jobID, count, err)
	}

	// Nomad rejects a scale request while a deployment is active, but not a job registration.
	// See Job.Scale in https://github.com/hashicorp/nomad/blob/v1.11.3/nomad/job_endpoint.go
	p.l.InfoContext(ctx, "an active deployment blocks scaling, registering the job with the new count",
		slog.String("job", jobID), slog.String("group", group), slog.Int("count", c))
	tg.Count = &c
	return p.register(ctx, job)
}

// register submits the job again. Nomad rejects it when the job changed since it was read.
func (p *Provider) register(ctx context.Context, job *api.Job) error {
	if _, _, err := p.Client.Jobs().EnforceRegister(job, deref(job.JobModifyIndex), p.writeOptions(ctx)); err != nil {
		return fmt.Errorf("cannot register job %q: %w", deref(job.ID), err)
	}
	return nil
}
