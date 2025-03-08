package sablier_test

import (
	"context"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/storetest"
	"go.uber.org/mock/gomock"
	"testing"
	"time"

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
					{Name: "apache", Status: sablier.InstanceStatusNotReady},
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
					{Name: "nginx-error", Status: sablier.InstanceStatusUnrecoverable, Message: "connection timeout"},
					{Name: "apache", Status: sablier.InstanceStatusReady},
				}),
			},
			want: false,
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

func setupSessionManager(t *testing.T) (sablier.Sablier, *storetest.MockStore, *providertest.MockProvider) {
	t.Helper()
	ctrl := gomock.NewController(t)

	p := providertest.NewMockProvider(ctrl)
	s := storetest.NewMockStore(ctrl)

	m := sablier.New(slogt.New(t), s, p)
	return m, s, p
}

func TestSessionsManager(t *testing.T) {
	t.Run("RemoveInstance", func(t *testing.T) {
		manager, store, _ := setupSessionManager(t)
		store.EXPECT().Delete(gomock.Any(), "test")
		err := manager.RemoveInstance(t.Context(), "test")
		assert.NilError(t, err)
	})
}

func TestSessionsManager_RequestReadySessionCancelledByUser(t *testing.T) {
	t.Run("request ready session is cancelled by user", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		manager, store, provider := setupSessionManager(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusNotReady}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.EXPECT().InstanceInspect(ctx, gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusNotReady}, nil)

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
		manager, store, provider := setupSessionManager(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusNotReady}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.EXPECT().InstanceInspect(t.Context(), gomock.Any()).Return(sablier.InstanceInfo{Name: "apache", Status: sablier.InstanceStatusNotReady}, nil)

		errchan := make(chan error)
		go func() {
			_, err := manager.RequestReadySession(t.Context(), []string{"apache"}, time.Minute, time.Second)
			errchan <- err
		}()

		assert.Error(t, <-errchan, "session was not ready after 1s")
	})
}

func TestSessionsManager_RequestReadySession(t *testing.T) {

	t.Run("request ready session is ready", func(t *testing.T) {
		manager, store, _ := setupSessionManager(t)
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
