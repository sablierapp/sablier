package nomad

import (
	"context"
	"log/slog"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// InstanceEvents turns the Job topic of the Nomad event stream into instance
// events. It reconnects with capped backoff and resynchronizes from a job listing.
func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wanted := make(map[provider.InstanceEventType]bool, len(opts.Types))
	for _, t := range opts.Types {
		wanted[t] = true
	}

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)
	go p.streamEvents(ctx, wanted, eventsC, errC)

	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

func (p *Provider) streamEvents(ctx context.Context, wanted map[provider.InstanceEventType]bool, eventsC chan<- sablier.InstanceEvent, errC chan<- error) {
	defer close(eventsC)
	defer close(errC)

	tr := newTracker(p.namespace)
	seeded := false
	backoff := p.backoffMin
	var index uint64

	for ctx.Err() == nil {
		// The first listing seeds the tracker silently. The later ones report
		// what changed while the stream was disconnected.
		jobs, lastIndex, err := p.listJobs(ctx)
		if err != nil {
			p.l.WarnContext(ctx, "cannot list jobs for the event stream, retrying", slog.Any("error", err), slog.Duration("backoff", backoff))
			if !sleepContext(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, p.backoffMax)
			continue
		}
		if seeded {
			if !p.emit(ctx, eventsC, wanted, tr.reconcile(jobs)) {
				return
			}
		} else {
			for _, job := range jobs {
				tr.seed(job)
			}
			seeded = true
		}
		if index == 0 {
			index = lastIndex
		}

		stream, err := p.Client.EventStream().Stream(ctx, map[api.Topic][]string{api.TopicJob: {"*"}}, index, p.queryOptions(ctx))
		if err != nil {
			p.l.WarnContext(ctx, "cannot open the Nomad event stream, retrying", slog.Any("error", err), slog.Duration("backoff", backoff))
			if !sleepContext(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, p.backoffMax)
			continue
		}
		backoff = p.backoffMin
		p.l.DebugContext(ctx, "nomad event stream connected", slog.Uint64("index", index))

		for batch := range stream {
			if batch.Err != nil {
				p.l.WarnContext(ctx, "nomad event stream interrupted", slog.Any("error", batch.Err))
				break
			}
			if batch.Index > 0 {
				index = batch.Index
			}
			for _, e := range batch.Events {
				if !p.emit(ctx, eventsC, wanted, p.handleEvent(ctx, tr, e)) {
					return
				}
			}
		}

		if ctx.Err() != nil {
			return
		}
		p.l.InfoContext(ctx, "reconnecting to the Nomad event stream", slog.Duration("backoff", backoff))
		if !sleepContext(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, p.backoffMax)
	}
}

// handleEvent maps one Nomad event onto instance events.
func (p *Provider) handleEvent(ctx context.Context, tr *tracker, e api.Event) []sablier.InstanceEvent {
	if e.Topic != api.TopicJob {
		return nil
	}
	job, deleted, err := e.DeregisteredJob()
	if err != nil || job == nil || job.ID == nil {
		p.l.DebugContext(ctx, "ignoring undecodable job event", slog.String("type", e.Type), slog.String("key", e.Key), slog.Any("error", err))
		return nil
	}
	// A purge or a garbage collection removes the job. A plain "nomad job stop"
	// only sets the Stop flag, which the tracker reports as stopped.
	if deleted || e.Type == "JobBatchDeregistered" {
		return tr.forgetJob(*job.ID)
	}
	return tr.observeJob(job)
}

// emit sends the wanted events and reports false once the context is done.
func (p *Provider) emit(ctx context.Context, eventsC chan<- sablier.InstanceEvent, wanted map[provider.InstanceEventType]bool, events []sablier.InstanceEvent) bool {
	for _, ev := range events {
		if len(wanted) > 0 && !wanted[ev.Type] {
			continue
		}
		p.l.DebugContext(ctx, "instance event", slog.String("type", string(ev.Type)), slog.String("name", ev.Info.Name))
		select {
		case eventsC <- ev:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
