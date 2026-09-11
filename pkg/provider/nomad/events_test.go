package nomad_test

import (
	"context"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func recvEvent(t *testing.T, events <-chan sablier.InstanceEvent) sablier.InstanceEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("event stream closed")
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for an instance event")
	}
	return sablier.InstanceEvent{}
}

func assertNoEvent(t *testing.T, events <-chan sablier.InstanceEvent) {
	t.Helper()
	select {
	case ev := <-events:
		t.Fatalf("unexpected event %s for %s", ev.Type, ev.Info.Name)
	case <-time.After(100 * time.Millisecond):
	}
}

// subscribe opens an event stream and closes it before the test ends, so the
// provider never logs to a finished test (the race detector reports that).
func subscribe(t *testing.T, p *nomad.Provider, opts provider.InstanceEventsOptions) (sablier.InstanceEventStream, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	stream := p.InstanceEvents(ctx, opts)
	t.Cleanup(func() {
		cancel()
		for range stream.Events {
		}
	})
	return stream, cancel
}

func TestInstanceEvents(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("whoami", 0, enabledMeta))
	p := m.provider(t)

	stream, cancel := subscribe(t, p, provider.InstanceEventsOptions{})
	waitStreamOpens(t, m, 1)

	t.Run("existing jobs do not emit events", func(t *testing.T) {
		assertNoEvent(t, stream.Events)
	})

	t.Run("scaling from zero emits started", func(t *testing.T) {
		m.pushJobEvent("JobRegistered", serviceJob("whoami", 1, enabledMeta), false)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventStarted)
		assert.Equal(t, ev.Info.Name, "whoami/web")
		assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStarting)
		assert.Equal(t, ev.Info.Enabled, "true")
		assert.Equal(t, ev.Info.Provider, sablier.ProviderNomad)
	})

	t.Run("stopping the job without purge emits stopped", func(t *testing.T) {
		stopped := serviceJob("whoami", 1, enabledMeta)
		stopped.Stop = new(true)
		m.pushJobEvent("JobDeregistered", stopped, false)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventStopped)
		assert.Equal(t, ev.Info.Name, "whoami/web")
		assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStopped)
	})

	t.Run("running the job again emits started", func(t *testing.T) {
		m.pushJobEvent("JobRegistered", serviceJob("whoami", 1, enabledMeta), false)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventStarted)
	})

	t.Run("scaling to zero emits stopped", func(t *testing.T) {
		m.pushJobEvent("JobRegistered", serviceJob("whoami", 0, enabledMeta), false)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	})

	t.Run("a new job emits created", func(t *testing.T) {
		m.pushJobEvent("JobRegistered", serviceJob("other", 0, enabledMeta), false)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventCreated)
		assert.Equal(t, ev.Info.Name, "other/web")
		assert.DeepEqual(t, ev.Info.Groups, []string{"default"})
	})

	t.Run("changing the labels emits updated", func(t *testing.T) {
		m.pushJobEvent("JobRegistered", serviceJob("other", 0, map[string]string{"sablier.enable": "true", "sablier.group": "team-a"}), false)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventUpdated)
		assert.DeepEqual(t, ev.Info.Groups, []string{"team-a"})
	})

	t.Run("purging the job emits removed", func(t *testing.T) {
		m.pushJobEvent("JobDeregistered", serviceJob("other", 0, enabledMeta), true)

		ev := recvEvent(t, stream.Events)
		assert.Equal(t, ev.Type, provider.InstanceEventRemoved)
		assert.Equal(t, ev.Info.Name, "other/web")
	})

	t.Run("jobs without the enable label are ignored", func(t *testing.T) {
		m.pushJobEvent("JobRegistered", serviceJob("unlabeled", 1, nil), false)

		assertNoEvent(t, stream.Events)
	})

	t.Run("cancelling the context closes the stream", func(t *testing.T) {
		cancel()

		select {
		case _, ok := <-stream.Events:
			assert.Assert(t, !ok, "events channel still open")
		case <-time.After(5 * time.Second):
			t.Fatal("events channel not closed")
		}
		select {
		case err, ok := <-stream.Err:
			assert.Assert(t, !ok, "unexpected terminal error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("error channel not closed")
		}
	})
}

func TestInstanceEvents_FiltersTypes(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("whoami", 0, enabledMeta))
	p := m.provider(t)

	stream, _ := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})
	waitStreamOpens(t, m, 1)

	m.pushJobEvent("JobRegistered", serviceJob("whoami", 1, enabledMeta), false)
	m.pushJobEvent("JobRegistered", serviceJob("whoami", 0, enabledMeta), false)

	ev := recvEvent(t, stream.Events)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
}

func TestInstanceEvents_ReconnectsAndResynchronizes(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("whoami", 0, enabledMeta))
	p := m.provider(t)

	stream, _ := subscribe(t, p, provider.InstanceEventsOptions{})
	waitStreamOpens(t, m, 1)

	// The job is scaled while the stream is down: no event is delivered for
	// it, so the change must be picked up by the resynchronization.
	m.addJob(serviceJob("whoami", 1, enabledMeta))
	m.closeStreams()

	ev := recvEvent(t, stream.Events)
	assert.Equal(t, ev.Type, provider.InstanceEventStarted)
	assert.Equal(t, ev.Info.Name, "whoami/web")
	waitStreamOpens(t, m, 2)

	// The reconnected stream delivers events again.
	m.pushJobEvent("JobRegistered", serviceJob("whoami", 0, enabledMeta), false)
	ev = recvEvent(t, stream.Events)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
}

func TestInstanceEvents_RetriesTheInitialConnection(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("whoami", 0, enabledMeta))
	m.setStreamFail(true)
	p := m.provider(t)

	stream, _ := subscribe(t, p, provider.InstanceEventsOptions{})
	time.Sleep(50 * time.Millisecond)
	m.setStreamFail(false)
	waitStreamOpens(t, m, 1)

	m.pushJobEvent("JobRegistered", serviceJob("whoami", 1, enabledMeta), false)

	ev := recvEvent(t, stream.Events)
	assert.Equal(t, ev.Type, provider.InstanceEventStarted)
}
