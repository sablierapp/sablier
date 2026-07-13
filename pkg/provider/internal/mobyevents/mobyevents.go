// Package mobyevents implements the shared event-stream loop for the
// providers that consume the Docker-compatible events API (Docker, Docker
// Swarm, and Podman through the Docker provider).
//
// It exists because the loop used to live as two verbatim copies (one per
// provider) that had already diverged: one retried forever, the other gave up
// after ten attempts and emitted a terminal error. This package is the single
// implementation with a single policy:
//
//   - The stream reconnects indefinitely with linear backoff (one second per
//     attempt, capped at 30 seconds). A daemon restart or network blip never
//     permanently kills the stream; the core's periodic reconciliation covers
//     whatever happened while disconnected.
//   - Both channels close only when ctx is cancelled. Err never receives a
//     value; it is part of the sablier.InstanceEventStream contract for
//     providers that can fail terminally, which the moby providers cannot.
//   - Every send is context-guarded so the goroutine exits when the consumer
//     is gone, instead of blocking forever on the channel and pinning the
//     events connection.
package mobyevents

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// DialFunc opens one connection to the provider's events API.
type DialFunc func(ctx context.Context) client.EventsResult

// BuildFunc converts a raw events API message into an instance event.
// It returns false when the message is not relevant to the subscription.
type BuildFunc func(ctx context.Context, msg events.Message) (sablier.InstanceEvent, bool)

// Stream opens the events API via dial and keeps consuming it until ctx is
// cancelled, redialing with linear backoff whenever the connection drops.
// See the package documentation for the exact reconnect and channel policy.
func Stream(ctx context.Context, l *slog.Logger, dial DialFunc, build BuildFunc) sablier.InstanceEventStream {
	return stream(ctx, l, dial, build, linearBackoff)
}

// stream is Stream with an injectable backoff so tests can run instantly.
func stream(ctx context.Context, l *slog.Logger, dial DialFunc, build BuildFunc, backoff func(attempt int) time.Duration) sablier.InstanceEventStream {
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

			if reconnect := consume(ctx, l, eventsC, dial(ctx), build); !reconnect {
				return
			}
		}
	}()
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// linearBackoff waits one more second per attempt, capped at 30 seconds.
func linearBackoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*time.Second, 30*time.Second)
}

// consume drains a single events connection, forwarding the instance events
// produced by build. It returns true when the connection ended and the caller
// should redial, or false when ctx was cancelled and the caller must stop.
func consume(
	ctx context.Context,
	l *slog.Logger,
	instance chan<- sablier.InstanceEvent,
	result client.EventsResult,
	build BuildFunc,
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
				// The send must be context-guarded: once the consumer stops
				// reading, a bare send blocks forever, leaking this goroutine
				// and its events connection.
				select {
				case instance <- event:
				case <-ctx.Done():
					return false
				}
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
