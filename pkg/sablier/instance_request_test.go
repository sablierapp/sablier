package sablier_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

// checkWithTimeout polls fn at the given interval until it returns true or the timeout expires.
func checkWithTimeout(interval, timeout time.Duration, fn func() bool) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if fn() {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-ticker.C:
		}
	}
}

func TestInstanceRequest_EmptyName(t *testing.T) {
	manager, _, _ := setupSablier(t)

	_, err := manager.InstanceRequest(t.Context(), "", time.Minute)
	assert.Error(t, err, "instance name cannot be empty")
}

func TestInstanceRequest_NewInstance_StartsAsync(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		return nil
	})

	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))
	assert.Equal(t, info.Name, "nginx")

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called asynchronously")
	}
}

func TestInstanceRequest_NewInstance_ReturnsBeforeStartCompletes(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startBlocking := make(chan struct{})
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	})

	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called")
	}

	close(startBlocking)
}

func TestInstanceRequest_DuplicateCallsDoNotHammerProvider(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startBlocking := make(chan struct{})
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	// First call: store miss -> triggers requestStart
	// Second call: store returns the not-ready state written by Put -> hits status != ready path
	gomock.InOrder(
		sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound),
		sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil),
	)

	// InstanceInspect: exactly once during requestStart (second call must not trigger another)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil).Times(1)

	// InstanceStart: exactly once (second call must not trigger another)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	}).Times(1)

	// Second call: start is still pending -> skips InstanceInspect, returns not-ready
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).Times(2)

	// First call — starts the goroutine
	info1, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info1.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Wait for the goroutine to enter InstanceStart
	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called")
	}

	// Second call — start still pending, skips inspect, no duplicate InstanceStart
	info2, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info2.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	close(startBlocking)
}

func TestInstanceRequest_AsyncErrorSurfacedOnNotReadyPath(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	// First call: store miss -> requestStart
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	// Goroutine fails immediately
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused"))

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Subsequent calls: store returns the stored not-ready state (realistic behavior).
	// While goroutine is still running, polling returns not-ready with Put.
	// Once goroutine finishes, consumePendingError surfaces the error (no Put in that case).
	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()

	assert.Assert(t, checkWithTimeout(100*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected async error to be surfaced on the not-ready path")
	assert.ErrorContains(t, err, "instance start failed: connection refused")
}

func TestInstanceRequest_RetryAfterErrorConsumed(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting
	secondDone := make(chan struct{})

	// All Get/Put/Inspect calls use AnyTimes since polling may hit them multiple times
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil).AnyTimes()

	gomock.InOrder(
		// First attempt — fails immediately
		provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused")),
		// Retry — succeeds
		provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
			close(secondDone)
			return nil
		}),
	)

	// 1st call: dispatches goroutine (fails)
	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	// 2nd call: poll until the error is consumable
	assert.Assert(t, checkWithTimeout(100*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected error to be surfaced")
	assert.ErrorContains(t, err, "instance start failed: connection refused")

	// 3rd call: entry cleared, store miss again -> requestStart retries
	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Retry goroutine was never started")
	}
}

func TestInstanceRequest_SuccessfulStartCleansUpPendingEntry(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting
	ready := sablier.InstanceInfo{Name: "nginx", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}

	// 1st call: store miss -> requestStart (goroutine succeeds)
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	startDone := make(chan struct{})
	gomock.InOrder(
		provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil), // pre-start inspect
		provider.EXPECT().InstanceInspect(ctx, "nginx").Return(ready, nil),       // post-start inspect in not-ready path
	)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startDone)
		return nil
	})

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Wait for goroutine to finish and self-clean
	select {
	case <-startDone:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutine never completed")
	}
	// Small settle time for the goroutine to acquire the lock and clean up
	time.Sleep(50 * time.Millisecond)

	// 2nd call: store returns not-ready, no pending entry exists, goes straight to inspect
	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil)
	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	info, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_StartTimeoutSurfacesError(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.InstanceStartTimeout = 100 * time.Millisecond
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	// InstanceStart blocks until context is cancelled
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(startCtx interface{}, _ string) error {
		<-startCtx.(interface{ Done() <-chan struct{} }).Done()
		return startCtx.(interface{ Err() error }).Err()
	})

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Subsequent calls: store returns not-ready; polling may get not-ready while
	// the goroutine is still in progress, or the timeout error once it completes.
	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected timeout error to be surfaced")
	assert.ErrorContains(t, err, "instance start failed")
}

func TestInstanceRequest_ExistingNotReady_InspectsProvider(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:   "nginx",
		Status: sablier.InstanceStatusStarting,
	}, nil)

	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}, nil)

	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_ExistingReady_SkipsInspect(t *testing.T) {
	manager, sessions, _ := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}, nil)

	sessions.EXPECT().Put(ctx, sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_StoreGetError(t *testing.T) {
	manager, sessions, _ := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, errors.New("connection refused"))

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.ErrorContains(t, err, "cannot retrieve instance from store")
}

func TestInstanceRequest_NewInstance_RecordsStartMetrics_Success(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	startDone := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startDone)
		return nil
	})

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	select {
	case <-startDone:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutine never completed")
	}
	// Settle for the goroutine to record the end metric.
	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		for _, c := range rec.snapshot() {
			if c == "start_end:nginx" {
				return true
			}
		}
		return false
	}), "expected start_end metric")

	calls := rec.snapshot()
	assertContains(t, calls, "ready_begin:nginx")
	assertContains(t, calls, "active+:nginx")
}

func TestInstanceRequest_NewInstance_RecordsStartFailure(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("boom"))

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err) // first call returns not-ready, error surfaces on next

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		for _, c := range rec.snapshot() {
			if c == "start_fail:nginx" {
				return true
			}
		}
		return false
	}), "expected start_fail metric")

	calls := rec.snapshot()
	for _, c := range calls {
		if c == "start_end:nginx" {
			t.Errorf("did not expect start_end on failure, got: %v", calls)
		}
	}
}

func TestInstanceRequest_ReadyTransition_RecordsReadyEnd(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	notReady := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting,
	}
	ready := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(ready, nil)
	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	// Pre-seed the ready-wait state by simulating a previous Begin.
	rec.RecordReadyWaitBegin("nginx")

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	calls := rec.snapshot()
	assertContains(t, calls, "ready_end:nginx")
}

func assertContains(t *testing.T, calls []string, want string) {
	t.Helper()
	for _, c := range calls {
		if c == want {
			return
		}
	}
	t.Errorf("expected %q in calls, got: %v", want, calls)
}

// readyAtMatcher is a gomock matcher that accepts any InstanceInfo whose
// Status is Ready and ReadyAt is non-nil. This is needed because ReadyAt is
// stamped with time.Now() inside InstanceRequest and cannot be predicted.
type readyAtMatcher struct{}

func (readyAtMatcher) Matches(x interface{}) bool {
	info, ok := x.(sablier.InstanceInfo)
	return ok && info.Status == sablier.InstanceStatusReady && info.ReadyAt != nil
}
func (readyAtMatcher) String() string {
	return "InstanceInfo{Status: ready, ReadyAt: non-nil}"
}

func TestInstanceRequest_ReadyAfter_StampsReadyAt(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startingInfo := sablier.InstanceInfo{
		Name: "nginx", Status: sablier.InstanceStatusStarting,
	}
	// Provider reports ready and carries the ReadyAfter label value.
	readyFromProvider := sablier.InstanceInfo{
		Name:       "nginx",
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: 100 * time.Millisecond,
	}

	// Store has a Starting entry — triggers the inspect path.
	sessions.EXPECT().Get(ctx, "nginx").Return(startingInfo, nil)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(readyFromProvider, nil)
	// Put must be called with a Ready state that has ReadyAt stamped.
	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusReady)
	assert.Assert(t, info.ReadyAt != nil, "ReadyAt should be stamped on first Ready transition")
	assert.Equal(t, info.ReadyAfter, 100*time.Millisecond)
	// Within the grace period — IsReady() should still return false.
	assert.Assert(t, !info.IsReady(), "IsReady() should be false within ReadyAfter grace period")
}

func TestInstanceRequest_ReadyAfter_GracePeriodRespectedByPolling(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.BlockingRefreshFrequency = 10 * time.Millisecond
	ctx := t.Context()

	startingInfo := sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting}
	readyFromProvider := sablier.InstanceInfo{
		Name:       "nginx",
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: 150 * time.Millisecond,
	}

	// First RequestSession call: store returns Starting → inspect → stamp ReadyAt.
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(startingInfo, nil).Times(1)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(readyFromProvider, nil).Times(1)
	sessions.EXPECT().Put(gomock.Any(), readyAtMatcher{}, time.Minute).Return(nil).Times(1)

	// Subsequent polling calls: store returns the stamped-Ready state.
	// ReadyAt is in the past by a growing amount; sessions.Put is called each tick.
	sessions.EXPECT().Get(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) (sablier.InstanceInfo, error) {
		// Simulate the store returning the previously stored Ready+ReadyAt state.
		past := time.Now().Add(-200 * time.Millisecond) // 200ms > 150ms ReadyAfter
		return sablier.InstanceInfo{
			Name:       "nginx",
			Status:     sablier.InstanceStatusReady,
			ReadyAfter: 150 * time.Millisecond,
			ReadyAt:    &past,
		}, nil
	}).AnyTimes()
	sessions.EXPECT().Put(gomock.Any(), gomock.Any(), time.Minute).Return(nil).AnyTimes()

	session, err := manager.RequestReadySession(ctx, []string{"nginx"}, time.Minute, 5*time.Second)
	assert.NilError(t, err)
	assert.Assert(t, session.IsReady())
}
