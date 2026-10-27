package sablier_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"go.uber.org/mock/gomock"

	"gotest.tools/v3/assert"
)

func TestSessionState_IsReady(t *testing.T) {
	type fields struct {
		Instances map[string]sablier.InstanceInfoWithError
		Error     error
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "all instances are ready",
			fields: fields{
				Instances: createMap([]sablier.InstanceInfo{
					{Name: "nginx", Status: sablier.InstanceStatusReady},
					{Name: "apache", Status: sablier.InstanceStatusReady},
				}),
			},
			want: true,
		},
		{
			name: "one instance is not ready",
			fields: fields{
				Instances: createMap([]sablier.InstanceInfo{
					{Name: "nginx", Status: sablier.InstanceStatusReady},
					{Name: "apache", Status: sablier.InstanceStatusStarting},
				}),
			},
			want: false,
		},
		{
			name: "no instances specified",
			fields: fields{
				Instances: createMap([]sablier.InstanceInfo{}),
			},
			want: true,
		},
		{
			name: "one instance has an error",
			fields: fields{
				Instances: createMap([]sablier.InstanceInfo{
					{Name: "nginx-error", Status: sablier.InstanceStatusError, Message: "connection timeout"},
					{Name: "apache", Status: sablier.InstanceStatusReady},
				}),
			},
			want: false,
		},
		{
			name: "ready instance within ReadyAfter grace period is not ready",
			fields: fields{
				Instances: createMap([]sablier.InstanceInfo{
					func() sablier.InstanceInfo {
						now := time.Now()
						return sablier.InstanceInfo{
							Name:       "nginx",
							Status:     sablier.InstanceStatusReady,
							ReadyAfter: time.Hour,
							ReadyAt:    &now,
						}
					}(),
				}),
			},
			want: false,
		},
		{
			name: "ready instance with elapsed ReadyAfter grace period is ready",
			fields: fields{
				Instances: createMap([]sablier.InstanceInfo{
					func() sablier.InstanceInfo {
						past := time.Now().Add(-2 * time.Second)
						return sablier.InstanceInfo{
							Name:       "nginx",
							Status:     sablier.InstanceStatusReady,
							ReadyAfter: time.Second,
							ReadyAt:    &past,
						}
					}(),
				}),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sablier.SessionState{
				Instances: tt.fields.Instances,
			}
			if got := s.IsReady(); got != tt.want {
				t.Errorf("SessionState.IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createMap(instances []sablier.InstanceInfo) map[string]sablier.InstanceInfoWithError {
	states := make(map[string]sablier.InstanceInfoWithError)

	for _, v := range instances {
		states[v.Name] = sablier.InstanceInfoWithError{
			Instance: v,
			Error:    nil,
		}
	}

	return states
}

func TestSessionsManager(t *testing.T) {
	t.Run("RemoveInstance", func(t *testing.T) {
		manager, store, _ := setupSablier(t)
		store.EXPECT().Delete(gomock.Any(), "test")
		err := manager.RemoveInstance(t.Context(), "test")
		assert.NilError(t, err)
	})
}

func TestRequestSession_RejectsUnlabeledInstances(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.WithRejectUnlabeledRequests(true)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 0,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusStopped,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)

	session, err := manager.RequestSession(ctx, []string{"nginx"}, time.Minute)
	assert.NilError(t, err)

	notManaged, ok := errors.AsType[sablier.ErrInstanceNotManaged](session.Instances["nginx"].Error)
	assert.Assert(t, ok)
	assert.Equal(t, notManaged.Name, "nginx")
}

// The "optional:" prefix is stripped before the name reaches the store and
// the provider, the session entry is keyed by the clean name, and the entry
// carries the Optional flag so it never gates readiness.
func TestRequestSession_OptionalPrefix(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 0,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusStopped,
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		return nil
	})
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	session, err := manager.RequestSession(ctx, []string{sablier.OptionalPrefix + "nginx"}, time.Minute)
	assert.NilError(t, err)

	entry, ok := session.Instances["nginx"]
	assert.Assert(t, ok, "session entry must be keyed by the clean name")
	assert.Assert(t, entry.Optional)
	// The instance is only starting, but as the sole (optional) member it
	// cannot gate the session.
	assert.Assert(t, session.IsReady())

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called asynchronously")
	}
}

func TestRequestSessionGroup_DoesNotRejectUnlabeledInstances(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.WithRejectUnlabeledRequests(true)
	manager.SetGroups(map[string][]string{"default": {"nginx"}})
	ctx := t.Context()
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 0,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusStopped,
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		return nil
	})
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	session, err := manager.RequestSessionGroup(ctx, "default", time.Minute)
	assert.NilError(t, err)
	assert.NilError(t, session.Instances["nginx"].Error)

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called asynchronously")
	}
}

// TestRequestSessionGroup_MultipleGroupsFiltering verifies that requesting a session
// for one group does NOT start instances from other groups, even when instances belong
// to multiple groups.
func TestRequestSessionGroup_MultipleGroupsFiltering(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	// Setup: team-a has [frontend, shared-api], team-b has [backend, shared-api]
	manager.SetGroups(map[string][]string{
		"team-a": {"frontend", "shared-api"},
		"team-b": {"backend", "shared-api"},
	})
	ctx := t.Context()

	stoppedInfo := func(name string) sablier.InstanceInfo {
		return sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusStopped,
		}
	}

	startingInfo := func(name string) sablier.InstanceInfo {
		return sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusStarting,
		}
	}

	// Expect only frontend and shared-api to be started (from team-a), NOT backend.
	// InstanceStart is called asynchronously in a goroutine; use a WaitGroup so
	// we don't exit the test before both calls have been observed.
	var startWg sync.WaitGroup
	startWg.Add(2)
	for _, name := range []string{"frontend", "shared-api"} {
		sessions.EXPECT().Get(ctx, name).Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
		provider.EXPECT().InstanceInspect(gomock.Any(), name).Return(stoppedInfo(name), nil)
		provider.EXPECT().InstanceStart(gomock.Any(), name).DoAndReturn(func(_ any, _ string) error {
			startWg.Done()
			return nil
		})
		sessions.EXPECT().Put(ctx, startingInfo(name), time.Minute).Return(nil)
	}

	// backend should NOT be called at all
	sessions.EXPECT().Get(ctx, "backend").Times(0)
	provider.EXPECT().InstanceInspect(gomock.Any(), "backend").Times(0)
	provider.EXPECT().InstanceStart(gomock.Any(), "backend").Times(0)

	session, err := manager.RequestSessionGroup(ctx, "team-a", time.Minute)
	assert.NilError(t, err)

	// Verify only team-a instances are in the session
	_, hasFrontend := session.Instances["frontend"]
	_, hasSharedAPI := session.Instances["shared-api"]
	_, hasBackend := session.Instances["backend"]

	assert.Assert(t, hasFrontend, "frontend should be in session")
	assert.Assert(t, hasSharedAPI, "shared-api should be in session")
	assert.Assert(t, !hasBackend, "backend should NOT be in session")

	// Wait for both async InstanceStart goroutines to complete so gomock can
	// verify all expected calls were made before the test exits.
	done := make(chan struct{})
	go func() { startWg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutines did not complete in time")
	}
}

func TestSessionsManager_RequestReadySessionCancelledByUser(t *testing.T) {
	t.Run("request ready session is cancelled by user", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		manager, store, provider := setupSablier(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.EXPECT().InstanceInspect(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil)

		errchan := make(chan error)
		go func() {
			_, err := manager.RequestReadySession(ctx, []string{"apache"}, time.Minute, time.Minute)
			errchan <- err
		}()

		// Cancel the call
		cancel()

		assert.Error(t, <-errchan, "request cancelled by user: context canceled")
	})
}

func TestSessionsManager_RequestReadySessionCancelledByTimeout(t *testing.T) {

	t.Run("request ready session is cancelled by timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			manager, store, provider := setupSablier(t)
			store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil).AnyTimes()
			store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			provider.EXPECT().InstanceInspect(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil).AnyTimes()

			errchan := make(chan error)
			go func() {
				_, err := manager.RequestReadySession(t.Context(), []string{"apache"}, time.Minute, time.Second)
				errchan <- err
			}()

			err := <-errchan
			timeoutErr, ok := errors.AsType[sablier.ErrTimeout](err)
			assert.Assert(t, ok)
			assert.Equal(t, time.Second, timeoutErr.Duration)
		})
	})
}

func TestSessionsManager_RequestReadySession(t *testing.T) {

	t.Run("request ready session is ready", func(t *testing.T) {
		manager, store, _ := setupSablier(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusReady}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		errchan := make(chan error)
		go func() {
			_, err := manager.RequestReadySession(context.Background(), []string{"apache"}, time.Minute, time.Second)
			errchan <- err
		}()

		assert.NilError(t, <-errchan)
	})
}

// setupSablierWithInMemoryStore uses a real session store, so each poll of a
// blocking request reads the state that the previous poll wrote.
func setupSablierWithInMemoryStore(t *testing.T) (*sablier.Sablier, *providertest.MockProvider) {
	t.Helper()
	p := providertest.NewMockProvider(gomock.NewController(t))
	p.EXPECT().InstanceDependencies(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	return sablier.New(slogt.New(t), inmemory.NewInMemory(), p), p
}

// A running instance must not wait a full refresh interval before the blocking
// request returns. See https://github.com/sablierapp/sablier/issues/282
func TestRequestReadySession_RunningInstanceDoesNotWaitRefreshFrequency(t *testing.T) {
	manager, provider := setupSablierWithInMemoryStore(t)
	manager.BlockingRefreshFrequency = 5 * time.Second

	running := sablier.InstanceInfo{Name: "whoami", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}
	provider.EXPECT().InstanceInspect(gomock.Any(), "whoami").Return(running, nil).AnyTimes()
	provider.EXPECT().InstanceStart(gomock.Any(), "whoami").Return(nil)

	begin := time.Now()
	session, err := manager.RequestReadySession(t.Context(), []string{"whoami"}, time.Minute, 30*time.Second)
	elapsed := time.Since(begin)

	assert.NilError(t, err)
	assert.Assert(t, session.IsReady())
	assert.Assert(t, elapsed < time.Second, "blocking request took %s", elapsed)
}

func TestRequestReadySession_ChecksAtLeastEveryRefreshFrequency(t *testing.T) {
	manager, provider := setupSablierWithInMemoryStore(t)
	manager.BlockingRefreshFrequency = 20 * time.Millisecond

	begin := time.Now()
	readyAt := begin.Add(1550 * time.Millisecond)
	provider.EXPECT().InstanceInspect(gomock.Any(), "whoami").DoAndReturn(func(_ context.Context, name string) (sablier.InstanceInfo, error) {
		if time.Now().Before(readyAt) {
			return sablier.InstanceInfo{Name: name, CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting}, nil
		}
		return sablier.InstanceInfo{Name: name, CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}, nil
	}).AnyTimes()
	provider.EXPECT().InstanceStart(gomock.Any(), "whoami").Return(nil)

	session, err := manager.RequestReadySession(t.Context(), []string{"whoami"}, time.Minute, 5*time.Second)
	elapsed := time.Since(begin)

	assert.NilError(t, err)
	assert.Assert(t, session.IsReady())
	assert.Assert(t, elapsed < 2100*time.Millisecond, "blocking request took %s", elapsed)
}

// The start is released between two backoff checks. The request must return
// when the start completes, not at the next backoff check.
func TestRequestReadySession_ChecksAgainWhenStartCompletes(t *testing.T) {
	manager, provider := setupSablierWithInMemoryStore(t)
	manager.BlockingRefreshFrequency = 5 * time.Second

	release := make(chan struct{})
	running := sablier.InstanceInfo{Name: "whoami", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}
	provider.EXPECT().InstanceInspect(gomock.Any(), "whoami").Return(running, nil).AnyTimes()
	provider.EXPECT().InstanceStart(gomock.Any(), "whoami").DoAndReturn(func(context.Context, string) error {
		<-release
		return nil
	})

	type result struct {
		session *sablier.SessionState
		err     error
	}
	results := make(chan result, 1)
	go func() {
		session, err := manager.RequestReadySession(t.Context(), []string{"whoami"}, time.Minute, 30*time.Second)
		results <- result{session, err}
	}()

	time.Sleep(1600 * time.Millisecond)
	close(release)
	releasedAt := time.Now()

	select {
	case r := <-results:
		assert.NilError(t, r.err)
		assert.Assert(t, r.session.IsReady())
		assert.Assert(t, time.Since(releasedAt) < 750*time.Millisecond, "blocking request returned %s after the start completed", time.Since(releasedAt))
	case <-time.After(10 * time.Second):
		t.Fatal("blocking request did not return")
	}
}

// An instance that stays starting must not cause a busy loop of readiness checks.
func TestRequestReadySession_DoesNotBusyLoop(t *testing.T) {
	tests := []struct {
		name             string
		refreshFrequency time.Duration
	}{
		{name: "after the start completes", refreshFrequency: 5 * time.Second},
		{name: "with a zero refresh frequency", refreshFrequency: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, provider := setupSablierWithInMemoryStore(t)
			manager.BlockingRefreshFrequency = tt.refreshFrequency

			var mu sync.Mutex
			inspects := 0
			starting := sablier.InstanceInfo{Name: "whoami", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting}
			provider.EXPECT().InstanceInspect(gomock.Any(), "whoami").DoAndReturn(func(context.Context, string) (sablier.InstanceInfo, error) {
				mu.Lock()
				defer mu.Unlock()
				inspects++
				return starting, nil
			}).AnyTimes()
			provider.EXPECT().InstanceStart(gomock.Any(), "whoami").Return(nil)

			_, err := manager.RequestReadySession(t.Context(), []string{"whoami"}, time.Minute, 450*time.Millisecond)

			_, ok := errors.AsType[sablier.ErrTimeout](err)
			assert.Assert(t, ok, "expected ErrTimeout, got %v", err)
			mu.Lock()
			defer mu.Unlock()
			assert.Assert(t, inspects < 10, "InstanceInspect was called %d times in 450ms", inspects)
		})
	}
}
