package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/webhook"
	"gotest.tools/v3/assert"
)

// capture records every HTTP POST received by a test server.
type capture struct {
	mu       sync.Mutex
	received []webhook.Payload
}

func (c *capture) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var p webhook.Payload
	_ = json.Unmarshal(body, &p)
	c.mu.Lock()
	c.received = append(c.received, p)
	c.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (c *capture) first(t *testing.T, timeout time.Duration) webhook.Payload {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		n := len(c.received)
		c.mu.Unlock()
		if n > 0 {
			c.mu.Lock()
			p := c.received[0]
			c.mu.Unlock()
			return p
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no webhook received within timeout")
	return webhook.Payload{}
}

func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.received)
}

// makeStream returns buffered channels wired into an InstanceEventStream.
func makeStream() (chan sablier.InstanceEvent, chan error, sablier.InstanceEventStream) {
	eventsC := make(chan sablier.InstanceEvent, 4)
	errC := make(chan error, 1)
	return eventsC, errC, sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

func TestDispatcher_FiresOnStartedEvent(t *testing.T) {
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(c.handler))
	defer srv.Close()

	d := webhook.NewDispatcher(
		[]config.WebhookEndpoint{{URL: srv.URL}},
		slogt.New(t),
	)

	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventStarted,
		Info: sablier.InstanceInfo{Name: "nginx"},
	}

	p := c.first(t, time.Second)
	assert.Equal(t, "started", p.Event)
	assert.Equal(t, "nginx", p.Instance.Name)
	assert.Assert(t, !p.Timestamp.IsZero())
}

func TestDispatcher_FiresOnStoppedEvent(t *testing.T) {
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(c.handler))
	defer srv.Close()

	d := webhook.NewDispatcher(
		[]config.WebhookEndpoint{{URL: srv.URL}},
		slogt.New(t),
	)

	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventStopped,
		Info: sablier.InstanceInfo{Name: "myapp"},
	}

	p := c.first(t, time.Second)
	assert.Equal(t, "stopped", p.Event)
	assert.Equal(t, "myapp", p.Instance.Name)
}

func TestDispatcher_IgnoresNonLifecycleEvents(t *testing.T) {
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(c.handler))
	defer srv.Close()

	d := webhook.NewDispatcher(
		[]config.WebhookEndpoint{{URL: srv.URL}},
		slogt.New(t),
	)

	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	// Send events that are not started/stopped.
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: "nginx"}}
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventUpdated, Info: sablier.InstanceInfo{Name: "nginx"}}
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: "nginx"}}
	// Then a started event as a sentinel to confirm the Watch loop is processing.
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "sentinel"}}

	c.first(t, time.Second) // wait for sentinel
	// Only the started sentinel should have been delivered.
	assert.Equal(t, 1, c.count())
}

func TestDispatcher_FiltersEventsByEndpointConfig(t *testing.T) {
	cStart := &capture{}
	srvStart := httptest.NewServer(http.HandlerFunc(cStart.handler))
	defer srvStart.Close()

	cStop := &capture{}
	srvStop := httptest.NewServer(http.HandlerFunc(cStop.handler))
	defer srvStop.Close()

	d := webhook.NewDispatcher([]config.WebhookEndpoint{
		{URL: srvStart.URL, Events: []string{"started"}},
		{URL: srvStop.URL, Events: []string{"stopped"}},
	}, slogt.New(t))

	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "app"}}
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: "app"}}

	// Wait until each server received exactly one event.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && (cStart.count() < 1 || cStop.count() < 1) {
		time.Sleep(5 * time.Millisecond)
	}

	assert.Equal(t, 1, cStart.count())
	assert.Equal(t, "started", cStart.received[0].Event)

	assert.Equal(t, 1, cStop.count())
	assert.Equal(t, "stopped", cStop.received[0].Event)
}

func TestDispatcher_ForwardsCustomHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := webhook.NewDispatcher([]config.WebhookEndpoint{
		{URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer secret"}},
	}, slogt.New(t))

	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "app"}}

	// Poll until the handler has been called.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && gotAuth == "" {
		time.Sleep(5 * time.Millisecond)
	}
	assert.Equal(t, "Bearer secret", gotAuth)
}

func TestDispatcher_HandlesHTTPError(t *testing.T) {
	// Endpoint that always returns 500 — dispatcher must not crash.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := webhook.NewDispatcher([]config.WebhookEndpoint{{URL: srv.URL}}, slogt.New(t))

	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "app"}}
	// Give the goroutine time to call the endpoint and log the error.
	time.Sleep(200 * time.Millisecond)
	// No assertion beyond "didn't crash".
}

func TestDispatcher_StopsOnContextCancel(t *testing.T) {
	d := webhook.NewDispatcher(nil, slogt.New(t))
	_, _, stream := makeStream()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		d.Watch(ctx, stream)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}

func TestDispatcher_StopsOnStreamClose(t *testing.T) {
	d := webhook.NewDispatcher(nil, slogt.New(t))
	eventsC, _, stream := makeStream()

	done := make(chan struct{})
	go func() {
		d.Watch(t.Context(), stream)
		close(done)
	}()

	close(eventsC)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after stream channel closed")
	}
}

func TestDispatcher_StopsOnStreamError(t *testing.T) {
	d := webhook.NewDispatcher(nil, slogt.New(t))
	_, errC, stream := makeStream()

	done := make(chan struct{})
	go func() {
		d.Watch(t.Context(), stream)
		close(done)
	}()

	errC <- io.ErrUnexpectedEOF
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after stream error")
	}
}

func TestDispatcher_MultipleEndpoints(t *testing.T) {
	const numEndpoints = 3
	captures := make([]*capture, numEndpoints)
	endpoints := make([]config.WebhookEndpoint, numEndpoints)
	servers := make([]*httptest.Server, numEndpoints)
	for i := range numEndpoints {
		captures[i] = &capture{}
		servers[i] = httptest.NewServer(http.HandlerFunc(captures[i].handler))
		defer servers[i].Close()
		endpoints[i] = config.WebhookEndpoint{URL: servers[i].URL}
	}

	d := webhook.NewDispatcher(endpoints, slogt.New(t))
	eventsC, _, stream := makeStream()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Watch(ctx, stream)

	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "app"}}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		allGot := true
		for _, c := range captures {
			if c.count() < 1 {
				allGot = false
				break
			}
		}
		if allGot {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	for i, c := range captures {
		assert.Equal(t, 1, c.count(), "endpoint %d did not receive the event", i)
	}
}
