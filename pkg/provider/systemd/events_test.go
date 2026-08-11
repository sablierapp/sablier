package systemd

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestUnitStatusChanged(t *testing.T) {
	status := func(state string) *dbus.UnitStatus {
		return &dbus.UnitStatus{ActiveState: state}
	}
	tests := []struct {
		name string
		u1   *dbus.UnitStatus
		u2   *dbus.UnitStatus
		want bool
	}{
		{
			name: "settle into active reports",
			u1:   status("inactive"),
			u2:   status("active"),
			want: true,
		},
		{
			name: "settle into inactive reports",
			u1:   status("active"),
			u2:   status("inactive"),
			want: true,
		},
		{
			name: "unchanged state reports nothing",
			u1:   status("active"),
			u2:   status("active"),
			want: false,
		},
		{
			name: "non-final new state reports nothing",
			u1:   status("active"),
			u2:   status("deactivating"),
			want: false,
		},
		{
			name: "activating to active reports settle",
			u1:   status("activating"),
			u2:   status("active"),
			want: true,
		},
		{
			name: "deactivating to inactive reports settle",
			u1:   status("deactivating"),
			u2:   status("inactive"),
			want: true,
		},
		{
			name: "reloading to active reports settle",
			u1:   status("reloading"),
			u2:   status("active"),
			want: true,
		},
		{
			name: "same transient reports nothing",
			u1:   status("activating"),
			u2:   status("activating"),
			want: false,
		},
		{
			name: "failed is settled",
			u1:   status("active"),
			u2:   status("failed"),
			want: true,
		},
		{
			name: "maintenance is settled",
			u1:   status("active"),
			u2:   status("maintenance"),
			want: true,
		},
		{
			name: "nil guards",
			u1:   nil,
			u2:   status("active"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, unitStatusChanged(tt.u1, tt.u2), tt.want)
		})
	}
}

func TestIsFinalState(t *testing.T) {
	for _, s := range []string{"activating", "deactivating", "reloading", "refreshing"} {
		assert.Assert(t, !isFinalState(s), "expected %q to be non-final", s)
	}
	for _, s := range []string{"active", "inactive", "failed", "maintenance", ""} {
		assert.Assert(t, isFinalState(s), "expected %q to be final", s)
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

func TestSystemdProvider_InstanceEvents_NewUnit(t *testing.T) {
	m := newMockSystemd(t, nil)
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	m.AddUnit(mockUnitConfig{name: "web.service", active: true})

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

func TestSystemdProvider_InstanceEvents_ReloadSettlesEmitsStarted(t *testing.T) {
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

	// Transient states are skipped; the settle back into active is reported
	// as a start.
	expectStarted(t, stream, "web.service")
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

func TestSystemdProvider_InstanceEvents_UnloadEmitsRemoved(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", active: true}})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventRemoved},
	})

	expectStarted(t, stream, "web.service")
	m.UnloadUnit("web.service")

	// systemd unloads inactive units, which removes them from the listing;
	// the subscription reports that as a removal.
	ev := expectEvent(t, stream)
	assert.Equal(t, ev.Type, provider.InstanceEventRemoved)
	assert.Equal(t, ev.Info.Name, "web.service")
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

func TestSystemdProvider_InstanceEvents_SkipsNotFoundUnits(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{
		{name: "ghost.service", loadState: "not-found"},
		{name: "web.service", active: true},
	})
	p := newProviderForTest(t, m, eventInterval, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventStopped},
	})

	// The baseline reports both units; the not-found ghost must be silently
	// skipped (no bogus stopped event, no failed inspect warning) while the
	// real unit still produces its started event.
	expectStarted(t, stream, "web.service")
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
