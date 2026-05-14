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

func TestStopAllUnregisteredInstances(t *testing.T) {
	s, sessions, p := setupSablier(t)

	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
	}

	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Get(ctx, "instance2").Return(sablier.InstanceInfo{
		Name:   "instance2",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(instances, nil)

	// Set up expectations for InstanceStop
	p.EXPECT().InstanceStop(ctx, "instance1").Return(nil)

	// Call the function under test
	err := s.StopAllUnregisteredInstances(ctx)
	assert.NilError(t, err)
}

func TestStopAllUnregisteredInstances_WithError(t *testing.T) {
	s, sessions, p := setupSablier(t)
	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
	}

	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Get(ctx, "instance2").Return(sablier.InstanceInfo{
		Name:   "instance2",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(instances, nil)

	// Set up expectations for InstanceStop with error
	p.EXPECT().InstanceStop(ctx, "instance1").Return(errors.New("stop error"))

	// Call the function under test
	err := s.StopAllUnregisteredInstances(ctx)
	assert.Error(t, err, "stop error")
}

// --- WatchAndStopExternallyStarted tests ---

// TestWatchAndStopExternallyStarted_StopsExternalInstance verifies that when a
// "started" event arrives for a Sablier-managed instance that is NOT in the
// sessions store (i.e. externally started), the instance is stopped.
func TestWatchAndStopExternallyStarted_StopsExternalInstance(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour // prevent ticker from firing

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceInfo, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// Instance is not in the sessions store
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)

	stopped := make(chan struct{})
	p.EXPECT().InstanceStop(gomock.Any(), "nginx").DoAndReturn(func(_ context.Context, _ string) error {
		close(stopped)
		return nil
	})

	// Send the event before starting the goroutine so the channel is pre-filled.
	eventsC <- sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting, Enabled: "true"}

	go s.WatchAndStopExternallyStarted(ctx)

	select {
	case <-stopped:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for externally-started instance to be stopped")
	}
}

// TestWatchAndStopExternallyStarted_SkipsSablierStartedInstance_InStore verifies
// that a "started" event for an instance already in the sessions store is
// not stopped (Sablier started it).
func TestWatchAndStopExternallyStarted_SkipsSablierStartedInstance_InStore(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceInfo, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// Instance IS in the sessions store → started by Sablier
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{
		Name:   "nginx",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// InstanceStop must NOT be called; gomock will fail the test if it is.
	eventsC <- sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting, Enabled: "true"}

	go s.WatchAndStopExternallyStarted(ctx)

	// Give the goroutine time to process the event then verify no stop happened.
	time.Sleep(100 * time.Millisecond)
}

// TestWatchAndStopExternallyStarted_SkipsNonSablierInstance verifies that a
// "started" event for an instance without sablier.enable=true is ignored.
func TestWatchAndStopExternallyStarted_SkipsNonSablierInstance(t *testing.T) {
	s, _, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceInfo, 1)
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	// No sessions.Get or InstanceStop expected.
	eventsC <- sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting, Enabled: "false"}

	go s.WatchAndStopExternallyStarted(ctx)

	time.Sleep(100 * time.Millisecond)
}

// TestWatchAndStopExternallyStarted_ReconciliationTicker verifies that even without
// incoming events, the periodic ticker triggers a reconciliation scan.
func TestWatchAndStopExternallyStarted_ReconciliationTicker(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 20 * time.Millisecond // fire quickly

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceInfo) // no events sent
	errC := make(chan error, 1)

	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	}).Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	instances := []sablier.InstanceConfiguration{{Name: "external"}}
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: false}).
		Return(instances, nil).AnyTimes()
	sessions.EXPECT().Get(gomock.Any(), "external").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()

	stopped := make(chan struct{}, 1)
	p.EXPECT().InstanceStop(gomock.Any(), "external").DoAndReturn(func(_ context.Context, _ string) error {
		select {
		case stopped <- struct{}{}:
		default:
		}
		return nil
	}).AnyTimes()

	go s.WatchAndStopExternallyStarted(ctx)

	select {
	case <-stopped:
		// success: reconciliation ticker fired and stopped the external instance
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconciliation ticker to stop external instance")
	}
}

// TestWatchAndStopExternallyStarted_EventStreamClosed verifies that when the
// started event stream closes, the function falls back to ticker-only mode
// and continues reconciliation.
func TestWatchAndStopExternallyStarted_EventStreamClosed(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.ExternallyStartedScanInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventsC := make(chan sablier.InstanceInfo)
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

	stopped := make(chan struct{}, 1)
	p.EXPECT().InstanceStop(gomock.Any(), "external").DoAndReturn(func(_ context.Context, _ string) error {
		select {
		case stopped <- struct{}{}:
		default:
		}
		return nil
	}).AnyTimes()

	go s.WatchAndStopExternallyStarted(ctx)

	select {
	case <-stopped:
		// success: reconciliation ticker still fires despite stream closure
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconciliation after stream closure")
	}
}
