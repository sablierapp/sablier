package docker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	wantStarted := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted)
	wantCreated := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventCreated)
	wantRemoved := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved)

	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("type", string(events.ContainerEventType))
		if wantStopped {
			filters.Add("event", "die")
		}
		if wantStarted {
			filters.Add("event", "start")
		}
		if wantCreated {
			filters.Add("event", "create")
		}
		if wantRemoved {
			filters.Add("event", "destroy")
		}
		return p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
	}
	build := func(ctx context.Context, msg events.Message) (sablier.InstanceEvent, bool) {
		name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		if name == "" {
			return sablier.InstanceEvent{}, false
		}
		switch msg.Action {
		case "die":
			if !wantStopped {
				return sablier.InstanceEvent{}, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after die event failed, using bare info", "container", name, "error", err)
				return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderDocker}}, true
			}
			return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}, true
		case "start":
			if !wantStarted {
				return sablier.InstanceEvent{}, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after start event failed, using bare info", "container", name, "error", err)
				return sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderDocker}}, true
			}
			return sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: info}, true
		case "create":
			if !wantCreated {
				return sablier.InstanceEvent{}, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after create event failed, using bare info", "container", name, "error", err)
				return sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderDocker}}, true
			}
			return sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: info}, true
		case "destroy":
			if !wantRemoved {
				return sablier.InstanceEvent{}, false
			}
			// Container is already gone; only the name is available.
			return sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderDocker}}, true
		default:
			return sablier.InstanceEvent{}, false
		}
	}
	return streamEvents(ctx, p.l, dial, build, linearBackoff)
}

func linearBackoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*time.Second, 30*time.Second)
}

// streamEvents runs the reconnect loop. dial is called on each connection attempt;
// backoff returns how long to wait before the next retry (return 0 in tests for instant retries).
func streamEvents(
	ctx context.Context,
	l *slog.Logger,
	dial func(ctx context.Context) client.EventsResult,
	build func(context.Context, events.Message) (sablier.InstanceEvent, bool),
	backoff func(attempt int) time.Duration,
) sablier.InstanceEventStream {
	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)
	go func() {
		defer close(eventsC)
		defer close(errC)
		for attempt := 0; ; attempt++ {
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
	}()
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// consumeEvents drains an EventsResult, forwarding instance events built by the
// build function. Returns true when the stream ended and the caller should
// reconnect, or false when the context was cancelled and the caller should stop.
func consumeEvents(
	ctx context.Context,
	l *slog.Logger,
	instance chan<- sablier.InstanceEvent,
	result client.EventsResult,
	build func(context.Context, events.Message) (sablier.InstanceEvent, bool),
) (reconnect bool) {
	for {
		select {
		case msg, ok := <-result.Messages:
			if !ok {
				l.WarnContext(ctx, "event stream closed")
				return true
			}
			l.DebugContext(ctx, "event received", "event", msg)
			if event, ok := build(ctx, msg); ok {
				instance <- event
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
