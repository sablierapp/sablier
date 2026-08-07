package sablier_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestWarmAllUnregisteredInstances(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.DefaultSessionDuration = 3 * time.Minute

	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
	}

	// instance1 has no session: looked up once by isStartedByUs and once by seedSession.
	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).Times(2)
	// instance2 already has a session: it must not be touched.
	sessions.EXPECT().Get(ctx, "instance2").Return(sablier.InstanceInfo{
		Name:   "instance2",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(instances, nil)

	// instance1 is running: it receives a session with the default session duration.
	ready := sablier.InstanceInfo{Name: "instance1", Status: sablier.InstanceStatusReady}
	p.EXPECT().InstanceInspect(ctx, "instance1").Return(ready, nil)
	sessions.EXPECT().Put(ctx, ready, 3*time.Minute).Return(nil)

	// Call the function under test
	err := s.WarmAllUnregisteredInstances(ctx)
	assert.NilError(t, err)
}

func TestWarmAllUnregisteredInstances_SkipsNotReadyInstance(t *testing.T) {
	s, sessions, p := setupSablier(t)

	ctx := t.Context()

	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
	}

	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).Times(2)

	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(instances, nil)

	// instance1 is not ready yet: no session is seeded (Put must not be called).
	p.EXPECT().InstanceInspect(ctx, "instance1").Return(sablier.InstanceInfo{
		Name:   "instance1",
		Status: sablier.InstanceStatusStarting,
	}, nil)

	err := s.WarmAllUnregisteredInstances(ctx)
	assert.NilError(t, err)
}

func TestWarmAllUnregisteredInstances_WithError(t *testing.T) {
	s, _, p := setupSablier(t)
	ctx := t.Context()

	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(nil, errors.New("list error"))

	err := s.WarmAllUnregisteredInstances(ctx)
	assert.Error(t, err, "list error")
}

// --- WatchAndWarmExternallyStarted tests ---

// startAutowarmWatcher launches s.WatchAndWarmExternallyStarted in a goroutine
// and registers a t.Cleanup that cancels the context and waits for the goroutine
// to exit. This prevents "Log called after test finished" panics from the slogt
// logger.
func startAutowarmWatcher(t *testing.T, s *sablier.Sablier, ctx context.Context, cancel context.CancelFunc) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		s.WatchAndWarmExternallyStarted(ctx)
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

// TestWatchAndWarmExternallyStarted_WarmsExternalInstance verifies that when a
// "started" event arrives for a Sablier-managed instance that is NOT in the
// sessions store (i.e. externally started), a session is seeded for it with
// the default session duration instead of stopping it.
func TestWatchAndWarmExternallyStarted_WarmsExternalInstance(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour // prevent ticker from firing
	s.DefaultSessionDuration = 3 * time.Minute

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// Instance is not in the sessions store: looked up by isStartedByUs and seedSession.
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).Times(2)

	ready := sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusReady, Enabled: "true"}
	p.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(ready, nil)

	seeded := make(chan struct{})
	sessions.EXPECT().Put(gomock.Any(), ready, 3*time.Minute).DoAndReturn(func(_ context.Context, _ sablier.InstanceInfo, _ time.Duration) error {
		close(seeded)
		return nil
	})

	// Send the event before starting the goroutine so the channel is pre-filled.
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusReady, Enabled: "true"}}

	startAutowarmWatcher(t, s, ctx, cancel)

	select {
	case <-seeded:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for externally-started instance to be warmed")
	}
}

// TestWatchAndWarmExternallyStarted_SkipsSablierStartedInstance_InStore verifies
// that a "started" event for an instance already in the sessions store is
// not touched (Sablier started it, or it already holds a session): the session
// must not be re-created, so it is never renewed in a loop.
func TestWatchAndWarmExternallyStarted_SkipsSablierStartedInstance_InStore(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// Instance IS in the sessions store → started by Sablier / already has a session
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{
		Name:   "nginx",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// Put must NOT be called; gomock will fail the test if it is.
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting, Enabled: "true"}}

	startAutowarmWatcher(t, s, ctx, cancel)

	// Give the goroutine time to process the event then verify no seed happened.
	time.Sleep(100 * time.Millisecond)
}

// TestWatchAndWarmExternallyStarted_DoesNotReseedWhenSessionAppearsBetweenChecks
// verifies the seedSession-level session lookup: if a session is registered
// between the isStartedByUs check and the seed (e.g. a concurrent RequestSession),
// no session is put on top of it.
func TestWatchAndWarmExternallyStarted_DoesNotReseedWhenSessionAppearsBetweenChecks(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// First lookup (isStartedByUs): no session yet. Second lookup (seedSession):
	// a session appeared in between. InstanceInspect and Put must NOT be called.
	first := sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{
		Name:   "nginx",
		Status: sablier.InstanceStatusReady,
	}, nil).After(first)

	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusReady, Enabled: "true"}}

	startAutowarmWatcher(t, s, ctx, cancel)

	time.Sleep(100 * time.Millisecond)
}

// TestWatchAndWarmExternallyStarted_SkipsPendingSablierStart verifies the
// pendingStarts half of the isStartedByUs guard: when Sablier itself has an
// in-progress start for an instance whose session entry is not written yet,
// the "started" event emitted by the provider must not trigger a seed.
func TestWatchAndWarmExternallyStarted_SkipsPendingSablierStart(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// Simulate the window between a Sablier-initiated start and its session
	// write: the store keeps answering "not found" for the whole test.
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()

	// InstanceRequest performs exactly one pre-start inspect; the warm watcher
	// must not add a second inspect (seedSession would).
	p.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(sablier.InstanceInfo{
		Name:    "nginx",
		Status:  sablier.InstanceStatusStopped,
		Enabled: "true",
	}, nil).Times(1)

	// A failed async start leaves the pendingStarts entry in place (it is only
	// consumed by the next request), which is exactly the state we need.
	p.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("start failed"))

	// The only allowed Put is the one from InstanceRequest itself.
	sessions.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	// Sablier initiates the start: nginx is now registered in pendingStarts.
	_, err := s.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	// The provider emits the started event for the Sablier-initiated start.
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusReady, Enabled: "true"}}

	startAutowarmWatcher(t, s, ctx, cancel)

	time.Sleep(100 * time.Millisecond)
}

// TestWarmAllUnregisteredInstances_SkipsOnInspectError verifies that an
// inspect failure during seeding is skipped without putting a session.
func TestWarmAllUnregisteredInstances_SkipsOnInspectError(t *testing.T) {
	s, sessions, p := setupSablier(t)
	ctx := t.Context()

	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return([]sablier.InstanceConfiguration{{Name: "instance1"}}, nil)

	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).Times(2)

	// Inspect fails: Put must NOT be called.
	p.EXPECT().InstanceInspect(ctx, "instance1").Return(sablier.InstanceInfo{}, errors.New("inspect error"))

	err := s.WarmAllUnregisteredInstances(ctx)
	assert.NilError(t, err)
}

// TestWarmAllUnregisteredInstances_SkipsOnSessionLookupError verifies that a
// store error (other than ErrKeyNotFound) on the seed-time lookup does not
// lead to seeding a session blindly.
func TestWarmAllUnregisteredInstances_SkipsOnSessionLookupError(t *testing.T) {
	s, sessions, p := setupSablier(t)
	ctx := t.Context()

	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return([]sablier.InstanceConfiguration{{Name: "instance1"}}, nil)

	// Both lookups fail with a store error: isStartedByUs treats it as "not started
	// by us", then seedSession bails out. InstanceInspect and Put must NOT be called.
	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, errors.New("store unavailable")).Times(2)

	err := s.WarmAllUnregisteredInstances(ctx)
	assert.NilError(t, err)
}

// TestWatchAndWarmExternallyStarted_SkipsNonSablierInstance verifies that a
// "started" event for an instance without sablier.enable=true is ignored.
func TestWatchAndWarmExternallyStarted_SkipsNonSablierInstance(t *testing.T) {
	s, _, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// No sessions.Get or Put expected.
	eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting, Enabled: "false"}}

	startAutowarmWatcher(t, s, ctx, cancel)

	time.Sleep(100 * time.Millisecond)
}

// TestWatchAndWarmExternallyStarted_ReconciliationTicker verifies that even without
// incoming events, the periodic ticker triggers a reconciliation scan that seeds
// sessions for unregistered running instances.
func TestWatchAndWarmExternallyStarted_ReconciliationTicker(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 20 * time.Millisecond // fire quickly

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent) // no events sent
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	instances := []sablier.InstanceConfiguration{{Name: "external"}}
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: false}).
		Return(instances, nil).AnyTimes()
	sessions.EXPECT().Get(gomock.Any(), "external").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	ready := sablier.InstanceInfo{Name: "external", Status: sablier.InstanceStatusReady}
	p.EXPECT().InstanceInspect(gomock.Any(), "external").Return(ready, nil).AnyTimes()

	seeded := make(chan struct{}, 1)
	sessions.EXPECT().Put(gomock.Any(), ready, gomock.Any()).DoAndReturn(func(_ context.Context, _ sablier.InstanceInfo, _ time.Duration) error {
		select {
		case seeded <- struct{}{}:
		default:
		}
		return nil
	}).AnyTimes()

	startAutowarmWatcher(t, s, ctx, cancel)

	select {
	case <-seeded:
		// success: reconciliation ticker fired and warmed the external instance
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconciliation ticker to warm external instance")
	}
}

// TestWatchAndWarmExternallyStarted_EventStreamClosed verifies that when the
// started event stream closes, the function falls back to ticker-only mode
// and continues reconciliation.
func TestWatchAndWarmExternallyStarted_EventStreamClosed(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// Close the events channel immediately to simulate stream closure.
	close(eventsC)

	instances := []sablier.InstanceConfiguration{{Name: "external"}}
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: false}).
		Return(instances, nil).AnyTimes()
	sessions.EXPECT().Get(gomock.Any(), "external").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	ready := sablier.InstanceInfo{Name: "external", Status: sablier.InstanceStatusReady}
	p.EXPECT().InstanceInspect(gomock.Any(), "external").Return(ready, nil).AnyTimes()

	seeded := make(chan struct{}, 1)
	sessions.EXPECT().Put(gomock.Any(), ready, gomock.Any()).DoAndReturn(func(_ context.Context, _ sablier.InstanceInfo, _ time.Duration) error {
		select {
		case seeded <- struct{}{}:
		default:
		}
		return nil
	}).AnyTimes()

	startAutowarmWatcher(t, s, ctx, cancel)

	select {
	case <-seeded:
		// success: reconciliation ticker still fires despite stream closure
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconciliation after stream closure")
	}
}
