package awsecs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/awsecs"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const pollInterval = 20 * time.Millisecond

func newEventsProvider(t *testing.T, f *fakeECS) *awsecs.Provider {
	t.Helper()
	p, err := awsecs.NewForTest(t.Context(), f, testCluster, slogt.New(t), pollInterval)
	assert.NilError(t, err)
	return p
}

// subscribe opens an event stream for the test. The cleanup cancels it and
// drains the events, so no event goroutine logs after the test ends.
func subscribe(t *testing.T, p *awsecs.Provider, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	stream := p.InstanceEvents(ctx, opts)
	t.Cleanup(func() {
		cancel()
		for range stream.Events {
		}
	})
	return stream
}

func expectEvent(t *testing.T, stream sablier.InstanceEventStream) sablier.InstanceEvent {
	t.Helper()
	select {
	case ev, ok := <-stream.Events:
		assert.Assert(t, ok, "events channel closed unexpectedly")
		return ev
	case err, ok := <-stream.Err:
		assert.Assert(t, ok, "error channel closed unexpectedly")
		t.Fatalf("unexpected stream error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
	}
	return sablier.InstanceEvent{}
}

func expectNoEvent(t *testing.T, stream sablier.InstanceEventStream) {
	t.Helper()
	select {
	case ev, ok := <-stream.Events:
		assert.Assert(t, ok, "events channel closed unexpectedly")
		t.Fatalf("unexpected event: %+v", ev)
	case err, ok := <-stream.Err:
		assert.Assert(t, ok, "error channel closed unexpectedly")
		t.Fatalf("unexpected stream error: %v", err)
	case <-time.After(10 * pollInterval):
	}
}

func expectClosed(t *testing.T, stream sablier.InstanceEventStream) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	eventsClosed, errClosed := false, false
	for !eventsClosed || !errClosed {
		select {
		case _, ok := <-stream.Events:
			if !ok {
				eventsClosed = true
			}
		case _, ok := <-stream.Err:
			if !ok {
				errClosed = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for stream channels to close")
		}
	}
}

func TestInstanceEvents_ContextCancel(t *testing.T) {
	t.Parallel()

	for _, wanted := range [][]provider.InstanceEventType{
		nil,
		{provider.InstanceEventStopped},
		{provider.InstanceEventStarted},
	} {
		p := newEventsProvider(t, newFake(newService("web", 1, 1, enabledTags(nil))))
		ctx, cancel := context.WithCancel(t.Context())
		stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{Types: wanted})
		cancel()
		expectClosed(t, stream)
	}
}

func TestInstanceEvents_Started(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 0, 0, enabledTags(nil)))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	// The initial scan is the baseline and emits nothing.
	expectNoEvent(t, stream)
	f.set(newService("web", 1, 0, enabledTags(nil)))

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStarted)
	assert.Equal(t, ev.Info.Name, "web")
	assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStarting)
	assert.Equal(t, ev.Info.Provider, sablier.ProviderECS)
}

func TestInstanceEvents_Stopped(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	expectNoEvent(t, stream)
	f.set(newService("web", 0, 1, enabledTags(nil)))

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	assert.Equal(t, ev.Info.Name, "web")
	assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStopped)
}

func TestInstanceEvents_Created(t *testing.T) {
	t.Parallel()

	f := newFake()
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated},
	})

	expectNoEvent(t, stream)
	f.set(newService("web", 1, 1, enabledTags(map[string]string{sablier.LabelGroup: "team-a"})))

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventCreated)
	assert.Equal(t, ev.Info.Name, "web")
	assert.DeepEqual(t, ev.Info.Groups, []string{"team-a"})
}

func TestInstanceEvents_Updated(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(map[string]string{sablier.LabelGroup: "team-a"})))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventUpdated},
	})

	expectNoEvent(t, stream)
	f.set(newService("web", 1, 1, enabledTags(map[string]string{sablier.LabelGroup: "team-b"})))

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventUpdated)
	assert.DeepEqual(t, ev.Info.Groups, []string{"team-b"})
}

func TestInstanceEvents_Removed(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventRemoved},
	})

	expectNoEvent(t, stream)
	f.remove("web")

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventRemoved)
	assert.Equal(t, ev.Info.Name, "web")
	assert.Equal(t, ev.Info.Provider, sablier.ProviderECS)
}

func TestInstanceEvents_RemovedEmitsStopped(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	expectNoEvent(t, stream)
	f.remove("web")

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	assert.Equal(t, ev.Info.Name, "web")
	assert.Equal(t, ev.Info.Status, sablier.InstanceStatusStopped)
}

func TestInstanceEvents_UnrelatedChangeEmitsNothing(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{})

	expectNoEvent(t, stream)
	// A running count change is not a lifecycle transition.
	f.set(newService("web", 1, 2, enabledTags(nil)))
	expectNoEvent(t, stream)
}

func TestInstanceEvents_TerminalErrorAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{})

	expectNoEvent(t, stream)
	f.setListErr(errors.New("throttled"))

	select {
	case err, ok := <-stream.Err:
		assert.Assert(t, ok, "expected a terminal error")
		assert.ErrorContains(t, err, "throttled")
	case ev := <-stream.Events:
		t.Fatalf("unexpected event: %+v", ev)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the terminal error")
	}
	expectClosed(t, stream)
}

func TestInstanceEvents_InitialScanFailureIsNotABaseline(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	f.setListErr(errors.New("throttled"))
	p := newEventsProvider(t, f)
	stream := subscribe(t, p, provider.InstanceEventsOptions{})

	// The first successful scan becomes the baseline without events.
	time.Sleep(2 * pollInterval)
	f.setListErr(nil)
	expectNoEvent(t, stream)

	f.set(newService("web", 0, 0, enabledTags(nil)))
	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
}
