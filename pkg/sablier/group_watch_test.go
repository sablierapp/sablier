package sablier_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestGroupWatch_ContextDone(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	cancel()

	done := make(chan struct{})
	go func() {
		s.GroupWatch(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("group watch did not stop on context cancellation")
	}
}

func TestGroupWatch_CreatedEvent_AddsToGroup(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventCreated,
		Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"web"}},
	}

	assert.Assert(t, pollFor(t, func() bool {
		return containsInstance(s.Groups(), "web", "nginx")
	}, 2*time.Second), "nginx should be in group web after created event")
}

func TestGroupWatch_RemovedEvent_RemovesFromGroup(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventRemoved,
		Info: sablier.InstanceInfo{Name: "nginx"},
	}

	assert.Assert(t, pollFor(t, func() bool {
		return !containsInstance(s.Groups(), "web", "nginx")
	}, 2*time.Second), "nginx should no longer be in group web after removed event")
}

func TestGroupWatch_UpdatedEvent_MovesGroup(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventUpdated,
		Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"api"}},
	}

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return !containsInstance(groups, "web", "nginx") && containsInstance(groups, "api", "nginx")
	}, 2*time.Second), "nginx should have moved from group web to group api after updated event")
}

func TestGroupWatch_ReconciliationUpdatesGroups(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	called := make(chan struct{}, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})

	p.EXPECT().InstanceGroups(gomock.Any()).DoAndReturn(func(context.Context) (map[string][]string, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return map[string][]string{"g": {"a", "b"}}, nil
	}).AnyTimes()

	go s.GroupWatch(ctx)

	select {
	case <-called:
		// at least one reconciliation happened (startup call)
	case <-time.After(3 * time.Second):
		t.Fatal("group watch did not call InstanceGroups")
	}

	assert.Assert(t, pollFor(t, func() bool {
		return containsInstance(s.Groups(), "g", "a") && containsInstance(s.Groups(), "g", "b")
	}, 2*time.Second), "expected groups to be populated via reconciliation")
}

func TestGroupWatch_EventStreamClosed_FallsBackToReconciliation(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)
	close(eventsC) // simulate immediate stream closure

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	called := make(chan struct{}, 1)
	p.EXPECT().InstanceGroups(gomock.Any()).DoAndReturn(func(context.Context) (map[string][]string, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return map[string][]string{"g": {"x"}}, nil
	}).AnyTimes()

	go s.GroupWatch(ctx)

	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Fatal("group watch did not call InstanceGroups after stream closure")
	}
}

func TestGroupWatch_ProviderErrorDoesNotUpdateGroups(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"existing": {"x"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	called := make(chan struct{}, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})

	p.EXPECT().InstanceGroups(gomock.Any()).DoAndReturn(func(context.Context) (map[string][]string, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil, errors.New("provider down")
	}).AnyTimes()

	go s.GroupWatch(ctx)

	select {
	case <-called:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("group watch did not poll provider")
	}

	time.Sleep(50 * time.Millisecond)
	assert.DeepEqual(t, s.Groups(), map[string][]string{"existing": {"x"}})
}

func TestGroupWatch_UpdatedEvent_LostLabel(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventUpdated,
		Info: sablier.InstanceInfo{Name: "nginx", Groups: nil},
	}

	assert.Assert(t, pollFor(t, func() bool {
		return !containsInstance(s.Groups(), "web", "nginx")
	}, 2*time.Second), "nginx should no longer be in group web after losing its label")
}

func TestGroupWatch_CreatedEvent_MovesToGroup(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventCreated,
		Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"api"}},
	}

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return !containsInstance(groups, "web", "nginx") && containsInstance(groups, "api", "nginx")
	}, 2*time.Second), "nginx should have moved from group web to group api after created event")
}

func TestGroupWatch_UpdatedEvent_GroupUnchanged(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventUpdated,
		Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"web"}},
	}

	// Give the event time to be processed then assert state is unchanged.
	time.Sleep(100 * time.Millisecond)
	groups := s.Groups()
	assert.Assert(t, containsInstance(groups, "web", "nginx"), "nginx should still be in group web")
	assert.Equal(t, len(groups["web"]), 1, "nginx should appear exactly once in group web")
}

func TestGroupWatch_ReconciliationMovesGroup(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{"api": {"nginx"}}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return !containsInstance(groups, "web", "nginx") && containsInstance(groups, "api", "nginx")
	}, 3*time.Second), "nginx should have moved from group web to group api via reconciliation")
}

func TestGroupWatch_ReconciliationRemovesFromGroup(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"web": {"nginx"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})
	// Provider no longer reports nginx in any group.
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	assert.Assert(t, pollFor(t, func() bool {
		return !containsInstance(s.Groups(), "web", "nginx")
	}, 3*time.Second), "nginx should have been removed from group web via reconciliation")
}

// TestGroupWatch_CreatedEvent_MultipleGroups verifies that an instance whose
// Groups field lists two groups is added to both groups simultaneously.
func TestGroupWatch_CreatedEvent_MultipleGroups(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	// shared-api belongs to both team-a and team-b.
	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventCreated,
		Info: sablier.InstanceInfo{Name: "shared-api", Groups: []string{"team-a", "team-b"}},
	}

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return containsInstance(groups, "team-a", "shared-api") &&
			containsInstance(groups, "team-b", "shared-api")
	}, 2*time.Second), "shared-api should appear in both team-a and team-b after created event")
}

// TestGroupWatch_UpdatedEvent_AddsSecondGroup verifies that updating an instance
// to add a second group leaves the first group intact and adds to the new one.
func TestGroupWatch_UpdatedEvent_AddsSecondGroup(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"team-a": {"shared-api"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	// Label updated from "team-a" to "team-a,team-b".
	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventUpdated,
		Info: sablier.InstanceInfo{Name: "shared-api", Groups: []string{"team-a", "team-b"}},
	}

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return containsInstance(groups, "team-a", "shared-api") &&
			containsInstance(groups, "team-b", "shared-api")
	}, 2*time.Second), "shared-api should be in both team-a and team-b after update")
}

// TestGroupWatch_RemovedEvent_RemovesFromAllGroups verifies that removing an
// instance that belonged to multiple groups drops it from every group.
func TestGroupWatch_RemovedEvent_RemovesFromAllGroups(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{
		"team-a": {"shared-api", "frontend"},
		"team-b": {"shared-api", "backend"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
	// Provider reflects the post-removal state: shared-api is gone; frontend and backend remain.
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{
		"team-a": {"frontend"},
		"team-b": {"backend"},
	}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventRemoved,
		Info: sablier.InstanceInfo{Name: "shared-api"},
	}

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return !containsInstance(groups, "team-a", "shared-api") &&
			!containsInstance(groups, "team-b", "shared-api")
	}, 2*time.Second), "shared-api should be removed from both team-a and team-b after removed event")

	// Other instances should be unaffected.
	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return containsInstance(groups, "team-a", "frontend") &&
			containsInstance(groups, "team-b", "backend")
	}, 2*time.Second), "frontend and backend should still be in their respective groups")
}

// TestGroupWatch_ReconciliationMultipleGroups verifies that reconciliation via
// InstanceGroups correctly places an instance into multiple groups.
func TestGroupWatch_ReconciliationMultipleGroups(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})
	p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{
		"team-a": {"frontend", "shared-api"},
		"team-b": {"backend", "shared-api"},
	}, nil).AnyTimes()

	go s.GroupWatch(ctx)

	assert.Assert(t, pollFor(t, func() bool {
		groups := s.Groups()
		return containsInstance(groups, "team-a", "shared-api") &&
			containsInstance(groups, "team-b", "shared-api") &&
			containsInstance(groups, "team-a", "frontend") &&
			containsInstance(groups, "team-b", "backend")
	}, 3*time.Second), "reconciliation should populate all instances across multiple groups")
}

// pollFor repeatedly calls check until it returns true or the deadline passes.
func pollFor(t *testing.T, check func() bool, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// containsInstance reports whether group contains instance.
func containsInstance(groups map[string][]string, group, instance string) bool {
	for _, m := range groups[group] {
		if m == instance {
			return true
		}
	}
	return false
}
