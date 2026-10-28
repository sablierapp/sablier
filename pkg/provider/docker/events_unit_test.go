package docker

// Unit tests for the events-API builder, in particular the one lifecycle change
// whose single message means two events. These run without a real Docker daemon
// (the daemon-backed event tests are in events_test.go).

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// stubEventsClient implements just enough of client.APIClient for InstanceEvents:
// the events stream to feed, and an inspect that fails. A failing inspect is the
// state a rename leaves the stream in — the container answers to its new name, so
// the old one it was asked about is gone — and it is also what the comment in
// `renamed` says the bare-info fallback is for.
type stubEventsClient struct {
	client.APIClient
	messages chan events.Message

	mu   sync.Mutex
	dial []client.Filters
}

func (s *stubEventsClient) Events(_ context.Context, options client.EventsListOptions) client.EventsResult {
	s.mu.Lock()
	s.dial = append(s.dial, options.Filters)
	s.mu.Unlock()
	return client.EventsResult{Messages: s.messages, Err: make(chan error, 1)}
}

func (s *stubEventsClient) ContainerInspect(_ context.Context, container string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{}, errors.New("no such container: " + container)
}

func (s *stubEventsClient) dialed() []client.Filters {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.dial)
}

// TestInstanceEvents_RenameMovesTheGroupFromTheTemporaryNameToTheFinalOne covers
// `docker compose up -d` recreating a container: compose creates the replacement
// as `<short-id>_<name>`, removes the container holding `<name>`, and only then
// renames the replacement to `<name>`. The stream has to report both halves, the
// removal first, or the group keeps a name no container has and every wake fails
// until the 30-second reconciliation.
func TestInstanceEvents_RenameMovesTheGroupFromTheTemporaryNameToTheFinalOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	const temporary = "1a58e0e8c0b3_demo"
	c := &stubEventsClient{messages: make(chan events.Message, 1)}
	p := &Provider{Client: c, l: slog.New(slog.NewTextHandler(io.Discard, nil))}

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{
			provider.InstanceEventCreated,
			provider.InstanceEventUpdated,
			provider.InstanceEventRemoved,
		},
	})

	c.messages <- events.Message{
		Type:   events.ContainerEventType,
		Action: events.ActionRename,
		Actor:  events.Actor{Attributes: map[string]string{"name": "/demo", "oldName": "/" + temporary}},
	}

	first := receive(t, stream.Events)
	second := receive(t, stream.Events)

	assert.Equal(t, first.Type, provider.InstanceEventRemoved)
	assert.Equal(t, first.Info.Name, temporary)
	assert.Equal(t, second.Type, provider.InstanceEventUpdated)
	assert.Equal(t, second.Info.Name, "demo")

	// The daemon has to have been asked for renames at all: without the filter the
	// two events above are messages the stream never receives, and the group keeps
	// the temporary name.
	dialed := c.dialed()
	assert.Equal(t, len(dialed), 1)
	assert.Assert(t, dialed[0]["event"][string(events.ActionRename)])
	assert.Assert(t, dialed[0]["event"][string(events.ActionDestroy)])
}

// TestInstanceEvents_RenameIsNotDialedForSubscribersThatCannotUseIt pins the other
// half of the filter: a subscriber that wants neither removals nor updates is not
// sent renames, which are only ever those two events.
func TestInstanceEvents_RenameIsNotDialedForSubscribersThatCannotUseIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	c := &stubEventsClient{messages: make(chan events.Message)}
	p := &Provider{Client: c, l: slog.New(slog.NewTextHandler(io.Discard, nil))}

	p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	// The dial happens on the stream's own goroutine; the message queue is empty and
	// unbuffered, so wait for the connection rather than assume it.
	deadline := time.Now().Add(5 * time.Second)
	for len(c.dialed()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	dialed := c.dialed()
	assert.Equal(t, len(dialed), 1)
	assert.Assert(t, !dialed[0]["event"][string(events.ActionRename)])
	assert.Assert(t, dialed[0]["event"][string(events.ActionStart)])
}

func receive(t *testing.T, events <-chan sablier.InstanceEvent) sablier.InstanceEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("no instance event arrived")
		return sablier.InstanceEvent{}
	}
}
