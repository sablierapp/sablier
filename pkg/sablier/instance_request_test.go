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

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		return nil
	})

	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))
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

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	})

	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

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

	notReady := sablier.NotReadyInstanceState("nginx", 0, 1)

	// First call: store miss → triggers requestStart
	// Second call: store returns the not-ready state written by Put → hits status != ready path
	gomock.InOrder(
		sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound),
		sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil),
	)

	// InstanceStart: exactly once (second call must not trigger another)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	}).Times(1)

	// Second call: start is still pending → skips InstanceInspect, returns not-ready
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).Times(2)

	// First call — starts the goroutine
	info1, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info1.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	// Wait for the goroutine to enter InstanceStart
	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called")
	}

	// Second call — start still pending, skips inspect, no duplicate InstanceStart
	info2, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info2.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	close(startBlocking)
}

func TestInstanceRequest_AsyncErrorSurfacedOnNotReadyPath(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	notReady := sablier.NotReadyInstanceState("nginx", 0, 1)

	// First call: store miss → requestStart
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	// Goroutine fails immediately
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused"))

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

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

	notReady := sablier.NotReadyInstanceState("nginx", 0, 1)
	secondDone := make(chan struct{})

	// All Get/Put calls use AnyTimes since polling may hit them multiple times
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()

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

	// 3rd call: entry cleared, store miss again → requestStart retries
	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Retry goroutine was never started")
	}
}

func TestInstanceRequest_SuccessfulStartCleansUpPendingEntry(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	notReady := sablier.NotReadyInstanceState("nginx", 0, 1)
	ready := sablier.ReadyInstanceState("nginx", 1)

	// 1st call: store miss → requestStart (goroutine succeeds)
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	startDone := make(chan struct{})
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startDone)
		return nil
	})

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

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
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(ready, nil)
	sessions.EXPECT().Put(ctx, ready, time.Minute).Return(nil)

	info, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_StartTimeoutSurfacesError(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.InstanceStartTimeout = 100 * time.Millisecond
	ctx := t.Context()

	notReady := sablier.NotReadyInstanceState("nginx", 0, 1)

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	// InstanceStart blocks until context is cancelled
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(startCtx interface{}, _ string) error {
		<-startCtx.(interface{ Done() <-chan struct{} }).Done()
		return startCtx.(interface{ Err() error }).Err()
	})

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

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
		Status: sablier.InstanceStatusNotReady,
	}, nil)

	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(sablier.InstanceInfo{
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
