package docker

// Unit tests for the reconnect loop in streamEvents.
// These run without a real Docker daemon by controlling the dial function directly.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// zeroBackoff disables all backoff delays so reconnect tests finish instantly.
func zeroBackoff(_ int) time.Duration { return 0 }

// makeResult is a convenience constructor for client.EventsResult.
func makeResult(msgs chan events.Message, errs chan error) client.EventsResult {
	return client.EventsResult{Messages: msgs, Err: errs}
}

func build(_ context.Context, msg events.Message) (sablier.InstanceEvent, bool) {
	name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	if name == "" {
		return sablier.InstanceEvent{}, false
	}
	return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped}}, true
}

// TestStreamEvents_Reconnect verifies that when the first connection drops
// (Messages channel closed), streamEvents transparently reconnects and continues
// delivering events from the second connection.
func TestStreamEvents_Reconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// attempt tracks which connection we're on (1-based).
	attempt := 0

	dial := func(_ context.Context) client.EventsResult {
		attempt++
		msgs := make(chan events.Message, 1)
		errs := make(chan error, 1)

		switch attempt {
		case 1:
			// First connection closes immediately -> triggers reconnect.
			close(msgs)
		default:
			// Second connection delivers one event; leave the channel open
			// so consumeEvents keeps running until the context is cancelled.
			msgs <- events.Message{
				Actor: events.Actor{
					Attributes: map[string]string{"name": "/web"},
				},
			}
		}

		return makeResult(msgs, errs)
	}

	stream := streamEvents(ctx, slogt.New(t), dial, build, zeroBackoff)

	select {
	case info, ok := <-stream.Events:
		assert.Assert(t, ok, "events channel closed unexpectedly")
		assert.Equal(t, info.Info.Name, "web")
		assert.Equal(t, string(info.Info.Status), string(sablier.InstanceStatusStopped))
		// Clean up: cancel the context so the goroutine exits.
		cancel()
	case err := <-stream.Err:
		t.Fatalf("unexpected error on Err channel: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for event (attempt reached %d)", attempt)
	}

	assert.Equal(t, attempt, 2, "expected exactly one reconnect (2 dial calls)")
}
