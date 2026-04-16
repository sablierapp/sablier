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

	// Wait for the async goroutine to complete
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

	// InstanceRequest returns immediately with not-ready
	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	// Verify the goroutine is still running (InstanceStart was called but blocked)
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

	// Both calls hit ErrKeyNotFound
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).Times(2)
	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil).Times(2)

	// InstanceStart must be called exactly once
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	}).Times(1)

	// First call — starts the goroutine
	info1, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info1.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	// Wait for the goroutine to actually enter InstanceStart
	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called")
	}

	// Second call — reuses existing pending start, does NOT call InstanceStart again
	info2, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info2.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	close(startBlocking)
}

func TestInstanceRequest_PreviousErrorForwardedToUser(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil).AnyTimes()

	// First call's goroutine fails immediately
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused")).Times(1)

	// First call succeeds (returns not-ready, goroutine starts)
	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	// Allow the goroutine to finish setting the error and closing ps.done
	assert.Assert(t, checkWithTimeout(100*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected error to be surfaced")
	assert.ErrorContains(t, err, "instance start failed: connection refused")
}

func TestInstanceRequest_RetryAfterErrorConsumed(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	secondDone := make(chan struct{})

	// Allow multiple Get calls: the polling helper may call InstanceRequest several times
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil).AnyTimes()

	gomock.InOrder(
		// First attempt — fails immediately
		provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused")),
		// Third call — retries with a new goroutine
		provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
			close(secondDone)
			return nil
		}),
	)

	// 1st call: dispatches goroutine
	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	// 2nd call: poll until the error is consumable (goroutine must finish first)
	assert.Assert(t, checkWithTimeout(100*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected error to be surfaced")
	assert.ErrorContains(t, err, "instance start failed: connection refused")

	// 3rd call: retries, starts a new goroutine
	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusNotReady))

	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Retry goroutine was never started")
	}
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
