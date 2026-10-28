package dockerswarm

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.opentelemetry.io/otel"
	"gotest.tools/v3/assert"
)

// fakeSwarm implements the events API and ServiceList over an in-memory set of
// services. Filters are ignored: the provider must not rely on them alone.
type fakeSwarm struct {
	client.APIClient
	msgs chan events.Message

	mu       sync.Mutex
	services map[string]swarm.Service
}

func newFakeSwarm(services ...swarm.Service) *fakeSwarm {
	f := &fakeSwarm{msgs: make(chan events.Message), services: make(map[string]swarm.Service)}
	for _, s := range services {
		f.services[s.Spec.Name] = s
	}
	return f
}

func (f *fakeSwarm) Events(_ context.Context, _ client.EventsListOptions) client.EventsResult {
	return client.EventsResult{Messages: f.msgs, Err: make(chan error)}
}

func (f *fakeSwarm) ServiceList(_ context.Context, _ client.ServiceListOptions) (client.ServiceListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []swarm.Service
	for _, s := range f.services {
		items = append(items, s)
	}
	return client.ServiceListResult{Items: items}, nil
}

func (f *fakeSwarm) put(s swarm.Service) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[s.Spec.Name] = s
}

func (f *fakeSwarm) delete(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.services, name)
}

func fakeService(name string, enabled bool, replicas uint64) swarm.Service {
	s := swarm.Service{ID: name + "-id"}
	s.Spec.Name = name
	if enabled {
		s.Spec.Labels = map[string]string{sablier.LabelEnable: "true"}
	}
	s.Spec.Mode = swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &replicas}}
	s.ServiceStatus = &swarm.ServiceStatus{DesiredTasks: replicas, RunningTasks: replicas}
	return s
}

func created(name string) events.Message {
	return events.Message{Action: events.ActionCreate, Actor: events.Actor{Attributes: map[string]string{"name": name}}}
}

func removed(name string) events.Message {
	return events.Message{Action: events.ActionRemove, Actor: events.Actor{Attributes: map[string]string{"name": name}}}
}

func scaled(name string, from, to int) events.Message {
	return events.Message{Action: events.ActionUpdate, Actor: events.Actor{Attributes: map[string]string{
		"name":         name,
		"replicas.old": strconv.Itoa(from),
		"replicas.new": strconv.Itoa(to),
	}}}
}

func startStream(t *testing.T, f *fakeSwarm, types ...provider.InstanceEventType) (func(events.Message), func() sablier.InstanceEvent) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	p := &Provider{Client: f, l: slog.New(slog.DiscardHandler), tracer: otel.Tracer("test")}
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{Types: types})

	send := func(msg events.Message) {
		t.Helper()
		select {
		case f.msgs <- msg:
		case <-ctx.Done():
			t.Fatalf("timed out sending %v", msg)
		}
	}
	next := func() sablier.InstanceEvent {
		t.Helper()
		select {
		case ev := <-stream.Events:
			return ev
		case <-ctx.Done():
			t.Fatal("timed out waiting for an event")
			return sablier.InstanceEvent{}
		}
	}
	return send, next
}

// Messages are built one at a time in order, so when the next event is for the
// managed service, the messages sent before it produced nothing.
func TestInstanceEvents_OnlyManagedServices(t *testing.T) {
	t.Parallel()
	f := newFakeSwarm(fakeService("managed", true, 1), fakeService("unmanaged", false, 1))
	send, next := startStream(t, f)

	send(scaled("unmanaged", 1, 0))
	send(scaled("managed", 1, 0))
	ev := next()
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	assert.Equal(t, ev.Info.Name, "managed")
	assert.Assert(t, ev.Info.IsEnabled())

	send(scaled("unmanaged", 0, 1))
	send(scaled("managed", 0, 1))
	ev = next()
	assert.Equal(t, ev.Type, provider.InstanceEventStarted)
	assert.Equal(t, ev.Info.Name, "managed")

	f.put(fakeService("unmanaged-new", false, 1))
	f.put(fakeService("managed-new", true, 1))
	send(created("unmanaged-new"))
	send(created("managed-new"))
	ev = next()
	assert.Equal(t, ev.Type, provider.InstanceEventCreated)
	assert.Equal(t, ev.Info.Name, "managed-new")

	f.delete("unmanaged")
	f.delete("unmanaged-new")
	send(removed("unmanaged"))
	send(removed("unmanaged-new"))
	send(removed("managed"))
	ev = next()
	assert.Equal(t, ev.Type, provider.InstanceEventRemoved)
	assert.Equal(t, ev.Info.Name, "managed")
	assert.Equal(t, ev.Info.Provider, sablier.ProviderSwarm)

	// A service created after the stream started is known from its create event.
	f.delete("managed-new")
	send(removed("managed-new"))
	ev = next()
	assert.Equal(t, ev.Type, provider.InstanceEventRemoved)
	assert.Equal(t, ev.Info.Name, "managed-new")
}

// The stopped-only stream (used to drop sessions) does not want created events,
// but must still learn about new managed services to report their removal.
func TestInstanceEvents_RemovalReportedAsStopped(t *testing.T) {
	t.Parallel()
	f := newFakeSwarm(fakeService("sentinel", true, 1))
	send, next := startStream(t, f, provider.InstanceEventStopped)

	f.put(fakeService("web", true, 1))
	send(created("web"))
	f.delete("web")
	send(removed("web"))
	ev := next()
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	assert.Equal(t, ev.Info.Name, "web")
	assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStopped)
}

func TestInstanceEvents_InspectFailure(t *testing.T) {
	t.Parallel()
	f := newFakeSwarm(fakeService("web", true, 1))
	send, next := startStream(t, f, provider.InstanceEventStopped)

	// Gone before it could be inspected: there is no telling it was managed.
	send(scaled("ghost", 1, 0))
	// Known to be managed: reported with the bare info from the event.
	f.delete("web")
	send(scaled("web", 1, 0))
	ev := next()
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	assert.Equal(t, ev.Info.Name, "web")
	assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStopped)
	assert.Equal(t, ev.Info.Provider, sablier.ProviderSwarm)
}

// A service that loses sablier.enable is no longer managed, including its removal.
func TestInstanceEvents_LabelRemoved(t *testing.T) {
	t.Parallel()
	f := newFakeSwarm(fakeService("web", true, 1), fakeService("sentinel", true, 1))
	send, next := startStream(t, f)

	f.put(fakeService("web", false, 0))
	send(scaled("web", 1, 0))
	f.delete("web")
	send(removed("web"))
	send(scaled("sentinel", 1, 0))
	ev := next()
	assert.Equal(t, ev.Info.Name, "sentinel")
}
