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
	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("scope", "swarm")
		filters.Add("type", "service")
		return p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
	}
	// InstanceEventStopped maps to: replicas scaled to 0, or service removed.
	extract := func(msg events.Message) string {
		if wantStopped && (msg.Actor.Attributes["replicas.new"] == "0" || msg.Action == "remove") {
			return msg.Actor.Attributes["name"]
		}
		return ""
	}
	return streamEvents(ctx, p.l, dial, extract, linearBackoff)
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
	extract func(events.Message) string,
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

			if reconnect := consumeEvents(ctx, l, eventsC, dial(ctx), extract); !reconnect {
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
	extract func(events.Message) string,
) (reconnect bool) {
	for {
		select {
		case msg, ok := <-result.Messages:
			if !ok {
				l.WarnContext(ctx, "event stream closed")
				return true
			}
			l.DebugContext(ctx, "event received", "event", msg)
			if name := extract(msg); name != "" {
				instance <- sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusNotReady}
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
