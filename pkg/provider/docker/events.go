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
	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("type", string(events.ContainerEventType))
		// Map InstanceEventStopped -> docker "die" event.
		// An empty Types slice means all event types.
		if len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped) {
			filters.Add("event", "die")
		}
		return p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
	}
	extract := func(msg events.Message) string {
		return strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	}
	return streamEvents(ctx, p.l, dial, extract, linearBackoff)
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
	extract func(events.Message) string,
	backoff func(attempt int) time.Duration,
) sablier.InstanceEventStream {
	eventsC := make(chan sablier.InstanceInfo)
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

			if reconnect := consumeEvents(ctx, l, eventsC, dial(ctx), extract); !reconnect {
				return
			}
		}
	}()
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// consumeEvents drains an EventsResult, forwarding instance info via the extract
// function. Returns true when the stream ended and the caller should reconnect,
// or false when the context was cancelled and the caller should stop.
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
