package systemd

import (
	"context"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestClassifyChange(t *testing.T) {
	tests := []struct {
		name        string
		prev        string
		prevSeen    bool
		activeState string
		baseline    bool
		wantStopped bool
		wantStarted bool
		wantCreated bool
		wantKinds   []provider.InstanceEventType
	}{
		{
			name:        "baseline active reports started",
			activeState: "active",
			baseline:    true,
			wantStarted: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventStarted},
		},
		{
			name:        "baseline inactive reports nothing",
			activeState: "inactive",
			baseline:    true,
			wantStopped: true,
		},
		{
			name:        "baseline without started filter",
			activeState: "active",
			baseline:    true,
		},
		{
			name:        "new active unit reports created and started",
			activeState: "active",
			wantStarted: true,
			wantCreated: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventStarted},
		},
		{
			name:        "new inactive unit reports created only",
			activeState: "inactive",
			wantStopped: true,
			wantCreated: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventCreated},
		},
		{
			name:        "new transient unit reports nothing",
			activeState: "activating",
			wantStarted: true,
			wantCreated: true,
		},
		{
			name:        "new unit without created filter reports started",
			activeState: "active",
			wantStarted: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventStarted},
		},
		{
			name:        "stop reports stopped",
			prev:        "active",
			prevSeen:    true,
			activeState: "inactive",
			wantStopped: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventStopped},
		},
		{
			name:        "start reports started",
			prev:        "inactive",
			prevSeen:    true,
			activeState: "active",
			wantStarted: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventStarted},
		},
		{
			name:        "unchanged state reports nothing",
			prev:        "active",
			prevSeen:    true,
			activeState: "active",
			wantStopped: true,
			wantStarted: true,
		},
		{
			name:        "transient state reports nothing",
			prev:        "active",
			prevSeen:    true,
			activeState: "activating",
			wantStopped: true,
			wantStarted: true,
		},
		{
			name:        "deactivating waits for stopped boundary",
			prev:        "active",
			prevSeen:    true,
			activeState: "deactivating",
			wantStopped: true,
		},
		{
			name:        "reloading does not leave running state",
			prev:        "active",
			prevSeen:    true,
			activeState: "reloading",
			wantStarted: true,
		},
		{
			name:        "failed state reports stopped",
			prev:        "active",
			prevSeen:    true,
			activeState: "failed",
			wantStopped: true,
			wantKinds:   []provider.InstanceEventType{provider.InstanceEventStopped},
		},
		{
			name:        "stop without stopped filter",
			prev:        "active",
			prevSeen:    true,
			activeState: "inactive",
			wantStarted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastState := make(map[string]lifecycleState)
			if tt.prevSeen {
				classifyChange("test.service", tt.prev, lastState, eventFilters{stopped: true, started: true, created: true}, true)
			}
			filters := eventFilters{stopped: tt.wantStopped, started: tt.wantStarted, created: tt.wantCreated}
			got := classifyChange("test.service", tt.activeState, lastState, filters, tt.baseline)
			assert.DeepEqual(t, got, tt.wantKinds)
		})
	}
}

const eventInterval = 50 * time.Millisecond

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
	case <-time.After(10 * eventInterval):
	}
}

func expectStarted(t *testing.T, stream sablier.InstanceEventStream, name string) {
	t.Helper()
	event := expectEvent(t, stream)
	assert.Equal(t, event.Type, provider.InstanceEventStarted)
	assert.Equal(t, event.Info.Name, name)
}

func TestSystemdProvider_InstanceEvents_BaselineStarted(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	event := expectEvent(t, stream)
	assert.Equal(t, event.Type, provider.InstanceEventStarted)
	assert.Equal(t, event.Info.Name, "web.service")
	assert.Equal(t, event.Info.Provider, sablier.ProviderSystemd)
}

func TestSystemdProvider_InstanceEvents_Started(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: false}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	// The initial inactive state is the subscription baseline.
	expectNoEvent(t, stream)
	m.SetActive("web.service", true)

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStarted)
	assert.Equal(t, ev.Info.Name, "web.service")
}

func TestSystemdProvider_InstanceEvents_Stopped(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventStopped},
	})

	expectStarted(t, stream, "web.service")
	m.SetActive("web.service", false)

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	assert.Equal(t, ev.Info.Name, "web.service")
}

func TestSystemdProvider_InstanceEvents_Created(t *testing.T) {
	m := newMockSystemd(t, nil)
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventStarted},
	})

	m.AddUnit(mockUnitConfig{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
	})

	// SubscribeUnitsContext has no empty-batch signal. The first unit it sees
	// is therefore the baseline, even though the manager began empty.
	expectStarted(t, stream, "web.service")
}

func TestSystemdProvider_InstanceEvents_DeactivatingEmitsOneStopped(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventStopped},
	})

	expectStarted(t, stream, "web.service")
	m.SetStatus("web.service", "deactivating")
	expectNoEvent(t, stream)
	m.SetStatus("web.service", "inactive")

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	expectNoEvent(t, stream)
}

func TestSystemdProvider_InstanceEvents_ReloadDoesNotEmitStarted(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	expectStarted(t, stream, "web.service")
	m.SetStatus("web.service", "reloading")
	expectNoEvent(t, stream)
	m.SetStatus("web.service", "active")
	expectNoEvent(t, stream)
}

func TestSystemdProvider_InstanceEvents_BaselineReloadingEmitsStartedWhenActive(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{
		{name: "decoy.service", active: true},
		{name: "web.service", status: "reloading"},
	})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	expectStarted(t, stream, "decoy.service")
	m.SetStatus("web.service", "active")
	expectStarted(t, stream, "web.service")
}

func TestSystemdProvider_InstanceEvents_BaselineDeactivatingEmitsStopped(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{
		{name: "decoy.service", active: true},
		{name: "web.service", status: "deactivating"},
	})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventStopped},
	})

	expectStarted(t, stream, "decoy.service")
	m.SetStatus("web.service", "inactive")
	event := expectEvent(t, stream)
	assert.Equal(t, event.Type, provider.InstanceEventStopped)
	assert.Equal(t, event.Info.Name, "web.service")
}

func TestSystemdProvider_InstanceEvents_UnloadRunningUnitEmitsStoppedNotRemoved(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
	}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventStopped, provider.InstanceEventRemoved},
	})

	expectStarted(t, stream, "web.service")
	m.UnloadUnit("web.service")
	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventStopped)
	expectNoEvent(t, stream)
}

func TestSystemdProvider_InstanceEvents_Removed(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventRemoved},
	})

	expectStarted(t, stream, "web.service")
	m.RemoveUnit("web.service")

	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventRemoved)
	assert.Equal(t, ev.Info.Name, "web.service")
}

func TestSystemdProvider_InstanceEvents_UnloadedNotRemoved(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		active:       true,
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
	}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventRemoved},
	})

	expectStarted(t, stream, "web.service")
	m.UnloadUnit("web.service")

	// systemd unloads inactive units, which removes them from the listing;
	// the unit file still exists, so no removed event may be emitted.
	expectNoEvent(t, stream)
}

func TestSystemdProvider_InstanceEvents_ContextCancel(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{})

	cancel()

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
