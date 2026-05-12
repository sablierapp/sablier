package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const maxReconnectAttempts = 10

func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	managedServices := map[string]struct{}{}
	if p.ignoreUnlabeled {
		services, err := p.managedServices(ctx)
		if err != nil {
			p.l.ErrorContext(ctx, "cannot list managed services", slog.Any("error", err))
		} else {
			managedServices = services
		}
	}
	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("scope", "swarm")
		filters.Add("type", "service")
		return p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
	}
	// InstanceEventStopped maps to: replicas scaled to 0, or service removed.
	// When scaled to 0 the service still exists and we can inspect it for full info.
	// When removed the service is gone; emit a bare stopped InstanceInfo instead.
	build := func(ctx context.Context, msg events.Message) (sablier.InstanceInfo, bool) {
		if !wantStopped {
			return sablier.InstanceInfo{}, false
		}
		name := msg.Actor.Attributes["name"]
		if name == "" {
			return sablier.InstanceInfo{}, false
		}
		if p.ignoreUnlabeled && !p.isManagedServiceEvent(ctx, managedServices, msg) {
			return sablier.InstanceInfo{}, false
		}
		if msg.Action == "remove" {
			delete(managedServices, msg.Actor.ID)
			delete(managedServices, name)
			// Service is gone; inspect would fail.
			return sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderSwarm}, true
		}
		if msg.Actor.Attributes["replicas.new"] == "0" {
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after scale-to-0 event failed, using bare info", "service", name, "error", err)
				return sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderSwarm}, true
			}
			return info, true
		}
		return sablier.InstanceInfo{}, false
	}
	return streamEvents(ctx, p.l, dial, build, linearBackoff)
}

func linearBackoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*time.Second, 30*time.Second)
}

// streamEvents runs the reconnect loop. dial is called on each connection attempt;
// backoff returns how long to wait before the next retry.
func streamEvents(
	ctx context.Context,
	l *slog.Logger,
	dial func(ctx context.Context) client.EventsResult,
	build func(context.Context, events.Message) (sablier.InstanceInfo, bool),
	backoff func(attempt int) time.Duration,
) sablier.InstanceEventStream {
	eventsC := make(chan sablier.InstanceInfo)
	errC := make(chan error, 1)
	go func() {
		defer close(eventsC)
		defer close(errC)
		for attempt := range maxReconnectAttempts + 1 {
			if attempt > 0 {
				d := backoff(attempt)
				l.WarnContext(ctx, "reconnecting event stream", "attempt", attempt, "backoff", d)
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return
				}
			}

			if reconnect := consumeEvents(ctx, l, eventsC, dial(ctx), build); !reconnect {
				return
			}
		}

		errC <- fmt.Errorf("event stream reconnect failed after %d attempts", maxReconnectAttempts)
	}()
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// consumeEvents drains a swarm EventsResult. Returns true if the caller should
// reconnect, false if the context was cancelled.
func consumeEvents(
	ctx context.Context,
	l *slog.Logger,
	instance chan<- sablier.InstanceInfo,
	result client.EventsResult,
	build func(context.Context, events.Message) (sablier.InstanceInfo, bool),
) (reconnect bool) {
	for {
		select {
		case msg, ok := <-result.Messages:
			if !ok {
				l.WarnContext(ctx, "event stream closed")
				return true
			}
			l.DebugContext(ctx, "event received", "event", msg)
			if info, ok := build(ctx, msg); ok {
				instance <- info
			}
		case err, ok := <-result.Err:
			if !ok || errors.Is(err, io.EOF) {
				l.WarnContext(ctx, "event stream closed")
				return true
			}
			l.ErrorContext(ctx, "event stream error", slog.Any("error", err))
			return true
		case <-ctx.Done():
			return false
		}
	}
}

func (p *Provider) managedServices(ctx context.Context) (map[string]struct{}, error) {
	filters := client.Filters{}
	filters.Add("label", "sablier.enable=true")

	services, err := p.Client.ServiceList(ctx, client.ServiceListOptions{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("cannot list services: %w", err)
	}

	managed := make(map[string]struct{}, len(services.Items)*2)
	for _, service := range services.Items {
		managed[service.ID] = struct{}{}
		managed[service.Spec.Name] = struct{}{}
	}
	return managed, nil
}

func (p *Provider) isManagedServiceEvent(ctx context.Context, managed map[string]struct{}, msg events.Message) bool {
	if msg.Action == "remove" {
		return isKnownManagedService(managed, msg)
	}

	result, err := p.Client.ServiceInspect(ctx, msg.Actor.ID, client.ServiceInspectOptions{})
	if err != nil {
		p.l.DebugContext(ctx, "cannot inspect service for label check", slog.String("service", msg.Actor.ID), slog.Any("error", err))
		return isKnownManagedService(managed, msg)
	}

	service := result.Service
	if service.Spec.Labels["sablier.enable"] == "true" {
		managed[service.ID] = struct{}{}
		managed[service.Spec.Name] = struct{}{}
		return true
	}

	delete(managed, service.ID)
	delete(managed, service.Spec.Name)
	return false
}

func isKnownManagedService(managed map[string]struct{}, msg events.Message) bool {
	if _, ok := managed[msg.Actor.ID]; ok {
		return true
	}
	if _, ok := managed[msg.Actor.Attributes["name"]]; ok {
		return true
	}
	return false
}
