package docker

// Daemon-free tests for the rename handling; events_test.go covers the rest
// against a real daemon.

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

// stubEventsClient feeds the events stream and records each dial's filters.
// Its inspect always fails, so events fall back to bare info.
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

// TestInstanceEvents_Rename covers the rename at the end of a Compose recreate
// (see #1136): the temporary name is removed, then the final name is updated.
func TestInstanceEvents_Rename(t *testing.T) {
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

	dialed := c.dialed()
	assert.Equal(t, len(dialed), 1)
	assert.Assert(t, dialed[0]["event"][string(events.ActionRename)])
	assert.Assert(t, dialed[0]["event"][string(events.ActionDestroy)])
}

// TestInstanceEvents_RenameNotDialed checks that a subscriber wanting neither
// removals nor updates does not receive renames.
func TestInstanceEvents_RenameNotDialed(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	c := &stubEventsClient{messages: make(chan events.Message)}
	p := &Provider{Client: c, l: slog.New(slog.NewTextHandler(io.Discard, nil))}

	p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	// The dial happens on the stream's goroutine.
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
