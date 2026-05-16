package sablier_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
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
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)

	session, err := manager.RequestSession(ctx, []string{"nginx"}, time.Minute)
	assert.NilError(t, err)

	notManaged, ok := errors.AsType[sablier.ErrInstanceNotManaged](session.Instances["nginx"].Error)
	assert.Assert(t, ok)
	assert.Equal(t, notManaged.Name, "nginx")
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
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
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

	// Expect only frontend and shared-api to be started (from team-a), NOT backend
	for _, name := range []string{"frontend", "shared-api"} {
		sessions.EXPECT().Get(ctx, name).Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
		provider.EXPECT().InstanceInspect(ctx, name).Return(stoppedInfo(name), nil)
		provider.EXPECT().InstanceStart(gomock.Any(), name).Return(nil)
		sessions.EXPECT().Put(ctx, startingInfo(name), time.Minute).Return(nil)
	}

	// backend should NOT be called at all
	sessions.EXPECT().Get(ctx, "backend").Times(0)
	provider.EXPECT().InstanceInspect(ctx, "backend").Times(0)
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
}

func TestSessionsManager_RequestReadySessionCancelledByUser(t *testing.T) {
	t.Run("request ready session is cancelled by user", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		manager, store, provider := setupSablier(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.EXPECT().InstanceInspect(ctx, gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil)

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
		manager, store, provider := setupSablier(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.EXPECT().InstanceInspect(t.Context(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusStarting}, nil)

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
