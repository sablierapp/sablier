package mobyevents

import (
	"context"
	"strings"
	"sync/atomic"
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

func makeResult(msgs chan events.Message, errs chan error) client.EventsResult {
	return client.EventsResult{Messages: msgs, Err: errs}
}

func buildStopped(_ context.Context, msg events.Message) (sablier.InstanceEvent, bool) {
	name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	if name == "" {
		return sablier.InstanceEvent{}, false
	}
	return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped}}, true
}

// TestStream_Reconnect verifies that when the first connection drops
// (Messages channel closed), the stream transparently redials and continues
// delivering events from the second connection.
func TestStream_Reconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

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
			// Second connection delivers one event; leave the channel open so
			// the loop keeps running until the context is cancelled.
			msgs <- events.Message{Actor: events.Actor{Attributes: map[string]string{"name": "/web"}}}
		}
		return makeResult(msgs, errs)
	}

	s := stream(ctx, slogt.New(t), dial, buildStopped, zeroBackoff)

	select {
	case info, ok := <-s.Events:
		assert.Assert(t, ok, "events channel closed unexpectedly")
		assert.Equal(t, info.Info.Name, "web")
		assert.Equal(t, string(info.Info.Status), string(sablier.InstanceStatusStopped))
		cancel()
	case err := <-s.Err:
		t.Fatalf("unexpected error on Err channel: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for event (attempt reached %d)", attempt)
	}

	assert.Equal(t, attempt, 2, "expected exactly one reconnect (2 dial calls)")
}

// TestStream_ExitsWhenConsumerGone pins the context-guarded send: the stream
// goroutine must exit on cancellation even when nobody reads Events anymore.
// With a bare send it blocks forever, leaking the goroutine and pinning the
// events connection (this was the behavior of both previous provider-local
// copies of the loop).
func TestStream_ExitsWhenConsumerGone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	dial := func(_ context.Context) client.EventsResult {
		msgs := make(chan events.Message, 1)
		errs := make(chan error, 1)
		// One event ready to deliver; the stream goroutine picks it up and
		// blocks on the send because nobody reads Events.
		msgs <- events.Message{Actor: events.Actor{Attributes: map[string]string{"name": "/web"}}}
		return makeResult(msgs, errs)
	}

	s := stream(ctx, slogt.New(t), dial, buildStopped, zeroBackoff)

	// Let the goroutine enter the blocked send, then cancel: it must exit
	// (observable as Err closing) without anyone draining Events.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case _, ok := <-s.Err:
		assert.Assert(t, !ok, "no error value expected, only channel closure")
	case <-time.After(2 * time.Second):
		t.Fatal("event goroutine leaked: still blocked on send after context cancellation")
	}
}

// TestStream_ReconnectsIndefinitely pins the reconnect policy: the stream
// never gives up, and Err never carries a value. (One of the two previous
// copies aborted after ten attempts with a terminal error, permanently
// killing event delivery after ~1 minute of daemon downtime.)
func TestStream_ReconnectsIndefinitely(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var attempts atomic.Int32
	dial := func(_ context.Context) client.EventsResult {
		n := attempts.Add(1)
		msgs := make(chan events.Message, 1)
		errs := make(chan error, 1)
		if n <= 15 {
			// Every early connection dies immediately.
			close(msgs)
		} else {
			// Well past the old ten-attempt budget: deliver an event.
			msgs <- events.Message{Actor: events.Actor{Attributes: map[string]string{"name": "/web"}}}
		}
		return makeResult(msgs, errs)
	}

	s := stream(ctx, slogt.New(t), dial, buildStopped, zeroBackoff)

	select {
	case info, ok := <-s.Events:
		assert.Assert(t, ok, "events channel closed unexpectedly")
		assert.Equal(t, info.Info.Name, "web")
	case err := <-s.Err:
		t.Fatalf("stream gave up with a terminal error after %d attempts: %v", attempts.Load(), err)
	case <-ctx.Done():
		t.Fatalf("timed out; attempts=%d", attempts.Load())
	}
	assert.Assert(t, attempts.Load() > 10, "expected the stream to outlive the old ten-attempt budget")
}
