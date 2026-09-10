package sablier_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
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
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventCreated,
			Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"web"}},
		}
		synctest.Wait()

		assert.Assert(t, containsInstance(s.Groups(), "web", "nginx"), "nginx should be in group web after created event")
	})
}

func TestGroupWatch_RemovedEvent_RemovesFromGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventRemoved,
			Info: sablier.InstanceInfo{Name: "nginx"},
		}
		synctest.Wait()

		assert.Assert(t, !containsInstance(s.Groups(), "web", "nginx"), "nginx should no longer be in group web after removed event")
	})
}

func TestGroupWatch_UpdatedEvent_MovesGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventUpdated,
			Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"api"}},
		}
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, !containsInstance(groups, "web", "nginx") && containsInstance(groups, "api", "nginx"),
			"nginx should have moved from group web to group api after updated event")
	})
}

func TestGroupWatch_ReconciliationUpdatesGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)

		ctx, cancel := context.WithCancel(t.Context())

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

		startGroupWatch(t, s, ctx, cancel)

		select {
		case <-called:
			// at least one reconciliation happened (startup call)
		case <-time.After(3 * time.Second):
			t.Fatal("group watch did not call InstanceGroups")
		}
		synctest.Wait()

		assert.Assert(t, containsInstance(s.Groups(), "g", "a") && containsInstance(s.Groups(), "g", "b"),
			"expected groups to be populated via reconciliation")
	})
}

func TestGroupWatch_EventStreamClosed_FallsBackToReconciliation(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())

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

	startGroupWatch(t, s, ctx, cancel)

	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Fatal("group watch did not call InstanceGroups after stream closure")
	}
}

func TestGroupWatch_ProviderErrorDoesNotUpdateGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"existing": {"x"}})

		ctx, cancel := context.WithCancel(t.Context())

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

		startGroupWatch(t, s, ctx, cancel)

		select {
		case <-called:
			cancel()
		case <-time.After(3 * time.Second):
			t.Fatal("group watch did not poll provider")
		}

		synctest.Wait()
		assert.DeepEqual(t, s.Groups(), map[string][]string{"existing": {"x"}})
	})
}

func TestGroupWatch_UpdatedEvent_LostLabel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventUpdated,
			Info: sablier.InstanceInfo{Name: "nginx", Groups: nil},
		}
		synctest.Wait()

		assert.Assert(t, !containsInstance(s.Groups(), "web", "nginx"), "nginx should no longer be in group web after losing its label")
	})
}

func TestGroupWatch_CreatedEvent_MovesToGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventCreated,
			Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"api"}},
		}
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, !containsInstance(groups, "web", "nginx") && containsInstance(groups, "api", "nginx"),
			"nginx should have moved from group web to group api after created event")
	})
}

func TestGroupWatch_UpdatedEvent_GroupUnchanged(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventUpdated,
			Info: sablier.InstanceInfo{Name: "nginx", Groups: []string{"web"}},
		}

		// Wait until the event is processed, then make sure that the state did not change.
		synctest.Wait()
		groups := s.Groups()
		assert.Assert(t, containsInstance(groups, "web", "nginx"), "nginx should still be in group web")
		assert.Equal(t, len(groups["web"]), 1, "nginx should appear exactly once in group web")
	})
}

func TestGroupWatch_ReconciliationMovesGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{"api": {"nginx"}}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, !containsInstance(groups, "web", "nginx") && containsInstance(groups, "api", "nginx"),
			"nginx should have moved from group web to group api via reconciliation")
	})
}

func TestGroupWatch_ReconciliationRemovesFromGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"web": {"nginx"}})

		ctx, cancel := context.WithCancel(t.Context())

		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})
		// Provider no longer reports nginx in any group.
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)
		synctest.Wait()

		assert.Assert(t, !containsInstance(s.Groups(), "web", "nginx"), "nginx should have been removed from group web via reconciliation")
	})
}

// TestGroupWatch_CreatedEvent_MultipleGroups verifies that an instance whose
// Groups field lists two groups is added to both groups simultaneously.
func TestGroupWatch_CreatedEvent_MultipleGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		// shared-api belongs to both team-a and team-b.
		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventCreated,
			Info: sablier.InstanceInfo{Name: "shared-api", Groups: []string{"team-a", "team-b"}},
		}
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, containsInstance(groups, "team-a", "shared-api") &&
			containsInstance(groups, "team-b", "shared-api"),
			"shared-api should appear in both team-a and team-b after created event")
	})
}

// TestGroupWatch_UpdatedEvent_AddsSecondGroup verifies that updating an instance
// to add a second group leaves the first group intact and adds to the new one.
func TestGroupWatch_UpdatedEvent_AddsSecondGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{"team-a": {"shared-api"}})

		ctx, cancel := context.WithCancel(t.Context())

		eventsC := make(chan sablier.InstanceEvent, 1)
		errC := make(chan error, 1)
		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)

		// Label updated from "team-a" to "team-a,team-b".
		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventUpdated,
			Info: sablier.InstanceInfo{Name: "shared-api", Groups: []string{"team-a", "team-b"}},
		}
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, containsInstance(groups, "team-a", "shared-api") &&
			containsInstance(groups, "team-b", "shared-api"),
			"shared-api should be in both team-a and team-b after update")
	})
}

// TestGroupWatch_RemovedEvent_RemovesFromAllGroups verifies that removing an
// instance that belonged to multiple groups drops it from every group.
func TestGroupWatch_RemovedEvent_RemovesFromAllGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.SetGroups(map[string][]string{
			"team-a": {"shared-api", "frontend"},
			"team-b": {"shared-api", "backend"},
		})

		ctx, cancel := context.WithCancel(t.Context())

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

		startGroupWatch(t, s, ctx, cancel)

		eventsC <- sablier.InstanceEvent{
			Type: provider.InstanceEventRemoved,
			Info: sablier.InstanceInfo{Name: "shared-api"},
		}
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, !containsInstance(groups, "team-a", "shared-api") &&
			!containsInstance(groups, "team-b", "shared-api"),
			"shared-api should be removed from both team-a and team-b after removed event")

		// Other instances should be unaffected.
		assert.Assert(t, containsInstance(groups, "team-a", "frontend") &&
			containsInstance(groups, "team-b", "backend"),
			"frontend and backend should still be in their respective groups")
	})
}

// TestGroupWatch_ReconciliationMultipleGroups verifies that reconciliation via
// InstanceGroups correctly places an instance into multiple groups.
func TestGroupWatch_ReconciliationMultipleGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)

		ctx, cancel := context.WithCancel(t.Context())

		p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventCreated, provider.InstanceEventUpdated, provider.InstanceEventRemoved},
		}).Return(sablier.InstanceEventStream{Events: make(chan sablier.InstanceEvent), Err: make(chan error, 1)})
		p.EXPECT().InstanceGroups(gomock.Any()).Return(map[string][]string{
			"team-a": {"frontend", "shared-api"},
			"team-b": {"backend", "shared-api"},
		}, nil).AnyTimes()

		startGroupWatch(t, s, ctx, cancel)
		synctest.Wait()

		groups := s.Groups()
		assert.Assert(t, containsInstance(groups, "team-a", "shared-api") &&
			containsInstance(groups, "team-b", "shared-api") &&
			containsInstance(groups, "team-a", "frontend") &&
			containsInstance(groups, "team-b", "backend"),
			"reconciliation should populate all instances across multiple groups")
	})
}

// startGroupWatch launches s.GroupWatch in a goroutine and registers a
// t.Cleanup that cancels the context and waits for GroupWatch to exit.
// This prevents "Log called after test finished" panics from the slogt logger.
func startGroupWatch(t *testing.T, s *sablier.Sablier, ctx context.Context, cancel context.CancelFunc) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		s.GroupWatch(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
}

// containsInstance reports whether group contains instance.
func containsInstance(groups map[string][]string, group, instance string) bool {
	return slices.Contains(groups[group], instance)
}
