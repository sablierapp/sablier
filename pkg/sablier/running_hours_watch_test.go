package sablier_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
)

// insideWindowSpec returns an HH:MM-HH:MM spec that contains the current
// minute with 2-minute margins on each side.
func insideWindowSpec() string {
	now := time.Now()
	start := now.Add(-2 * time.Minute)
	end := now.Add(5 * time.Minute)
	return fmt.Sprintf("%02d:%02d-%02d:%02d", start.Hour(), start.Minute(), end.Hour(), end.Minute())
}

// outsideWindowSpec returns an HH:MM-HH:MM spec that ended ~4 hours ago and
// therefore cannot contain the current minute.
func outsideWindowSpec() string {
	base := time.Now().Add(-4 * time.Hour)
	end := base.Add(2 * time.Minute)
	return fmt.Sprintf("%02d:%02d-%02d:%02d", base.Hour(), base.Minute(), end.Hour(), end.Minute())
}

// TestWatchRunningHours_StartsInstanceInsideWindow verifies that when an
// instance's running-hours window is currently active, the watcher triggers
// an InstanceRequest which extends/creates the session.
func TestWatchRunningHours_StartsInstanceInsideWindow(t *testing.T) {
	s, sessions, p := setupSablier(t)
	// Large ticker: we rely on the immediate reconcile call at watcher startup.
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	spec := insideWindowSpec()

	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		Return(sablier.InstanceInfo{Name: "myapp", RunningHours: spec, Status: sablier.InstanceStatusReady}, nil)

	// InstanceRequest path: instance is already in the store (running) → extend TTL.
	sessions.EXPECT().Get(gomock.Any(), "myapp").
		Return(sablier.InstanceInfo{Name: "myapp", Status: sablier.InstanceStatusReady, RunningHours: spec}, nil)

	put := make(chan time.Duration, 1)
	sessions.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ sablier.InstanceInfo, ttl time.Duration) error {
			put <- ttl
			return nil
		})

	go s.WatchRunningHours(ctx)

	select {
	case ttl := <-put:
		if ttl <= 0 {
			t.Fatalf("expected a positive TTL from the running-hours extension, got %s", ttl)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for session to be created/extended")
	}
}

// TestWatchRunningHours_SkipsInstanceOutsideWindow verifies that when the
// current time is outside the running-hours window, no InstanceRequest is made.
func TestWatchRunningHours_SkipsInstanceOutsideWindow(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	spec := outsideWindowSpec()

	inspected := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		DoAndReturn(func(_ context.Context, _ string) (sablier.InstanceInfo, error) {
			close(inspected)
			return sablier.InstanceInfo{Name: "myapp", RunningHours: spec}, nil
		})

	// sessions.Get and sessions.Put must NOT be called (gomock strict mode enforces this).

	go s.WatchRunningHours(ctx)

	select {
	case <-inspected:
		// reconcile ran; no further calls expected
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to run")
	}
}

// TestWatchRunningHours_SkipsNonEnabledInstance verifies that instances whose
// Enabled field is not "true" are skipped before InstanceInspect is called.
func TestWatchRunningHours_SkipsNonEnabledInstance(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	listed := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		DoAndReturn(func(_ context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
			close(listed)
			return []sablier.InstanceConfiguration{{Name: "myapp", Enabled: "false"}}, nil
		})

	// InstanceInspect must NOT be called (gomock strict mode enforces this).

	go s.WatchRunningHours(ctx)

	select {
	case <-listed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to run")
	}
}

// TestWatchRunningHours_SkipsInstanceWithoutRunningHoursLabel verifies that
// instances without the running-hours label are skipped after InstanceInspect.
func TestWatchRunningHours_SkipsInstanceWithoutRunningHoursLabel(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	inspected := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		DoAndReturn(func(_ context.Context, _ string) (sablier.InstanceInfo, error) {
			close(inspected)
			return sablier.InstanceInfo{Name: "myapp", RunningHours: ""}, nil
		})

	// sessions.Get and sessions.Put must NOT be called.

	go s.WatchRunningHours(ctx)

	select {
	case <-inspected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to run")
	}
}

// TestWatchRunningHours_HandlesInstanceListError verifies that when InstanceList
// returns an error the reconciler logs and returns without crashing.
func TestWatchRunningHours_HandlesInstanceListError(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	listed := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		DoAndReturn(func(_ context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
			close(listed)
			return nil, errors.New("provider unavailable")
		})

	go s.WatchRunningHours(ctx)

	select {
	case <-listed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to run")
	}
}

// TestWatchRunningHours_HandlesInstanceInspectError verifies that when
// InstanceInspect fails the reconciler logs a warning and skips the instance.
func TestWatchRunningHours_HandlesInstanceInspectError(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	inspected := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		DoAndReturn(func(_ context.Context, _ string) (sablier.InstanceInfo, error) {
			close(inspected)
			return sablier.InstanceInfo{}, errors.New("inspect failed")
		})

	// sessions.Get and sessions.Put must NOT be called.

	go s.WatchRunningHours(ctx)

	select {
	case <-inspected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to run")
	}
}

// TestWatchRunningHours_HandlesInvalidRunningHoursLabel verifies that an
// unparseable running-hours label is skipped with a warning, without crashing.
func TestWatchRunningHours_HandlesInvalidRunningHoursLabel(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	inspected := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		DoAndReturn(func(_ context.Context, _ string) (sablier.InstanceInfo, error) {
			close(inspected)
			return sablier.InstanceInfo{Name: "myapp", RunningHours: "not-a-valid-spec"}, nil
		})

	// sessions.Get and sessions.Put must NOT be called.

	go s.WatchRunningHours(ctx)

	select {
	case <-inspected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to run")
	}
}

// TestWatchRunningHours_TickerTriggersReconciliation verifies that after the
// initial reconcile, the watcher fires another reconcile on each tick.
func TestWatchRunningHours_TickerTriggersReconciliation(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	spec := insideWindowSpec()

	// Expect at least 2 calls (initial + at least one tick).
	second := make(chan struct{})
	calls := 0
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		DoAndReturn(func(_ context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
			calls++
			if calls == 2 {
				close(second)
			}
			return []sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil
		}).AnyTimes()

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		Return(sablier.InstanceInfo{Name: "myapp", RunningHours: spec, Status: sablier.InstanceStatusReady}, nil).AnyTimes()

	sessions.EXPECT().Get(gomock.Any(), "myapp").
		Return(sablier.InstanceInfo{Name: "myapp", Status: sablier.InstanceStatusReady, RunningHours: spec}, nil).AnyTimes()

	sessions.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	go s.WatchRunningHours(ctx)

	select {
	case <-second:
		// at least two reconcile cycles ran
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ticker to trigger a second reconciliation")
	}
}

// TestWatchRunningHours_StopsOnContextCancellation verifies that the watcher
// exits cleanly when its context is cancelled.
func TestWatchRunningHours_StopsOnContextCancellation(t *testing.T) {
	s, _, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())

	listed := make(chan struct{})
	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		DoAndReturn(func(_ context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
			close(listed)
			return []sablier.InstanceConfiguration{}, nil
		})

	done := make(chan struct{})
	go func() {
		s.WatchRunningHours(ctx)
		close(done)
	}()

	// Wait for the first reconcile to run, then cancel.
	select {
	case <-listed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for initial reconcile")
	}

	cancel()

	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("WatchRunningHours did not stop after context cancellation")
	}
}

// TestWatchRunningHours_MultipleInstances_MixedWindow verifies that when
// multiple instances are present, only those inside their window are started.
func TestWatchRunningHours_MultipleInstances_MixedWindow(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	inSpec := insideWindowSpec()
	outSpec := outsideWindowSpec()

	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{
			{Name: "active-app", Enabled: "true"},
			{Name: "inactive-app", Enabled: "true"},
		}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "active-app").
		Return(sablier.InstanceInfo{Name: "active-app", RunningHours: inSpec, Status: sablier.InstanceStatusReady}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "inactive-app").
		Return(sablier.InstanceInfo{Name: "inactive-app", RunningHours: outSpec}, nil)

	// Only active-app should reach InstanceRequest → sessions.Get + sessions.Put.
	sessions.EXPECT().Get(gomock.Any(), "active-app").
		Return(sablier.InstanceInfo{Name: "active-app", Status: sablier.InstanceStatusReady, RunningHours: inSpec}, nil)

	put := make(chan struct{})
	sessions.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ sablier.InstanceInfo, _ time.Duration) error {
			close(put)
			return nil
		})

	// sessions.Get for inactive-app must NOT be called.

	go s.WatchRunningHours(ctx)

	select {
	case <-put:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for active-app session to be extended")
	}
}

// TestWatchRunningHours_InstanceRequestError verifies that when InstanceRequest
// returns an error (e.g. store failure) the reconciler logs a warning and does
// not crash. Non-ErrKeyNotFound errors from sessions.Get cause InstanceRequest
// to return immediately without spawning any async goroutine.
func TestWatchRunningHours_InstanceRequestError(t *testing.T) {
	s, sessions, p := setupSablier(t)
	s.RunningHoursRefreshFrequency = 24 * time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	spec := insideWindowSpec()

	p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: true}).
		Return([]sablier.InstanceConfiguration{{Name: "myapp", Enabled: "true"}}, nil)

	p.EXPECT().InstanceInspect(gomock.Any(), "myapp").
		Return(sablier.InstanceInfo{Name: "myapp", RunningHours: spec}, nil)

	// A non-ErrKeyNotFound error from sessions.Get causes InstanceRequest to
	// return an error immediately (no async goroutine is dispatched).
	storeErr := make(chan struct{})
	sessions.EXPECT().Get(gomock.Any(), "myapp").
		DoAndReturn(func(_ context.Context, _ string) (sablier.InstanceInfo, error) {
			close(storeErr)
			return sablier.InstanceInfo{}, errors.New("store unavailable")
		})

	// sessions.Put and InstanceStart must NOT be called.

	go s.WatchRunningHours(ctx)

	select {
	case <-storeErr:
		// Reconcile ran and hit the InstanceRequest error path; no crash expected.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to encounter InstanceRequest error")
	}
}
