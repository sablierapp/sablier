package awsecs

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// maxConsecutivePollErrors is the number of consecutive scan failures after
// which InstanceEvents closes the stream with a terminal error.
const maxConsecutivePollErrors = 5

// InstanceEvents polls the cluster and emits an event for each lifecycle
// change of an enabled service. ECS publishes service changes to EventBridge
// only, so the provider compares snapshots instead of reading a stream.
func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)

	want := func(t provider.InstanceEventType) bool {
		return len(opts.Types) == 0 || slices.Contains(opts.Types, t)
	}

	go func() {
		defer close(eventsC)
		defer close(errC)

		// known is nil until a scan succeeds. The first successful scan is the
		// baseline and emits no event.
		known, err := p.snapshot(ctx)
		if err != nil {
			p.l.ErrorContext(ctx, "initial service scan failed", slog.Any("error", err))
		}

		ticker := time.NewTicker(p.pollInterval)
		defer ticker.Stop()

		var consecutiveErrors int
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			current, err := p.snapshot(ctx)
			if err != nil {
				consecutiveErrors++
				p.l.WarnContext(ctx, "service scan failed during polling",
					slog.Any("error", err),
					slog.Int("consecutive_errors", consecutiveErrors),
				)
				if consecutiveErrors >= maxConsecutivePollErrors {
					p.l.ErrorContext(ctx, "too many consecutive poll errors, closing event stream",
						slog.Int("max", maxConsecutivePollErrors))
					errC <- err
					return
				}
				continue
			}
			consecutiveErrors = 0

			if known != nil {
				for _, ev := range p.diff(ctx, known, current, want) {
					select {
					case eventsC <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
			known = current
		}
	}()

	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// snapshot returns the enabled services keyed by service name.
func (p *Provider) snapshot(ctx context.Context) (map[string]types.Service, error) {
	services, err := p.listEnabledServices(ctx)
	if err != nil {
		return nil, err
	}
	snap := make(map[string]types.Service, len(services))
	for _, svc := range services {
		snap[aws.ToString(svc.ServiceName)] = svc
	}
	return snap, nil
}

// diff compares two snapshots and returns the wanted events in name order.
func (p *Provider) diff(ctx context.Context, before, after map[string]types.Service, want func(provider.InstanceEventType) bool) []sablier.InstanceEvent {
	var events []sablier.InstanceEvent

	for _, name := range slices.Sorted(maps.Keys(after)) {
		svc := after[name]
		prev, seen := before[name]
		if !seen {
			if want(provider.InstanceEventCreated) {
				events = append(events, p.event(ctx, provider.InstanceEventCreated, svc))
			}
			continue
		}
		if want(provider.InstanceEventUpdated) && !maps.Equal(tagsToLabels(prev.Tags), tagsToLabels(svc.Tags)) {
			events = append(events, p.event(ctx, provider.InstanceEventUpdated, svc))
		}
		wasRunning, isRunning := prev.DesiredCount > 0, svc.DesiredCount > 0
		switch {
		case isRunning && !wasRunning && want(provider.InstanceEventStarted):
			events = append(events, p.event(ctx, provider.InstanceEventStarted, svc))
		case wasRunning && !isRunning && want(provider.InstanceEventStopped):
			events = append(events, p.event(ctx, provider.InstanceEventStopped, svc))
		}
	}

	for _, name := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[name]; ok {
			continue
		}
		// The service is gone or no longer enabled. Only the name is known.
		bare := sablier.InstanceInfo{Name: name, Provider: sablier.ProviderECS}
		switch {
		case want(provider.InstanceEventRemoved):
			events = append(events, sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: bare})
		case want(provider.InstanceEventStopped):
			bare.Status = sablier.InstanceStatusStopped
			events = append(events, sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: bare})
		}
	}

	return events
}

// event builds an event from the described service.
func (p *Provider) event(ctx context.Context, t provider.InstanceEventType, svc types.Service) sablier.InstanceEvent {
	info, err := serviceToInfo(&svc)
	if err != nil {
		p.l.WarnContext(ctx, "cannot build the instance info for the event, using bare info",
			slog.String("service", aws.ToString(svc.ServiceName)),
			slog.Any("error", err),
		)
		info = sablier.InstanceInfo{Name: aws.ToString(svc.ServiceName), Provider: sablier.ProviderECS}
	}
	return sablier.InstanceEvent{Type: t, Info: info}
}
