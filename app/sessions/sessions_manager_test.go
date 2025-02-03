package sessions

import (
	"context"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/store/storetest"
	"go.uber.org/mock/gomock"
	"testing"
	"time"

	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/sessions/mocks"
	"github.com/stretchr/testify/mock"
	"gotest.tools/v3/assert"
)

func TestSessionState_IsReady(t *testing.T) {
	type fields struct {
		Instances map[string]InstanceState
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
				Instances: createMap([]instance.State{
					{Name: "nginx", Status: instance.Ready},
					{Name: "apache", Status: instance.Ready},
				}),
			},
			want: true,
		},
		{
			name: "one instance is not ready",
			fields: fields{
				Instances: createMap([]instance.State{
					{Name: "nginx", Status: instance.Ready},
					{Name: "apache", Status: instance.NotReady},
				}),
			},
			want: false,
		},
		{
			name: "no instances specified",
			fields: fields{
				Instances: createMap([]instance.State{}),
			},
			want: true,
		},
		{
			name: "one instance has an error",
			fields: fields{
				Instances: createMap([]instance.State{
					{Name: "nginx-error", Status: instance.Unrecoverable, Message: "connection timeout"},
					{Name: "apache", Status: instance.Ready},
				}),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SessionState{
				Instances: tt.fields.Instances,
			}
			if got := s.IsReady(); got != tt.want {
				t.Errorf("SessionState.IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createMap(instances []instance.State) map[string]InstanceState {
	states := make(map[string]InstanceState)

	for _, v := range instances {
		states[v.Name] = InstanceState{
			Instance: v,
			Error:    nil,
		}
	}

	return states
}

func setupSessionManager(t *testing.T) (Manager, *storetest.MockStore, *mocks.ProviderMock) {
	t.Helper()
	ctrl := gomock.NewController(t)

	p := mocks.NewProviderMock()
	s := storetest.NewMockStore(ctrl)

	m := NewSessionsManager(slogt.New(t), s, p)
	return m, s, p
}

func TestSessionsManager(t *testing.T) {
	t.Run("RemoveInstance", func(t *testing.T) {
		manager, store, _ := setupSessionManager(t)
		store.EXPECT().Delete(gomock.Any(), "test")
		err := manager.RemoveInstance("test")
		assert.NilError(t, err)
	})
}

func TestSessionsManager_RequestReadySessionCancelledByUser(t *testing.T) {
	t.Run("request ready session is cancelled by user", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		manager, store, provider := setupSessionManager(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(instance.State{Name: "apache", Status: instance.NotReady}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.On("GetState", mock.Anything).Return(instance.State{Name: "apache", Status: instance.NotReady}, nil)

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
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(instance.State{Name: "apache", Status: instance.NotReady}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		provider.On("GetState", mock.Anything).Return(instance.State{Name: "apache", Status: instance.NotReady}, nil)

		errchan := make(chan error)
		go func() {
			_, err := manager.RequestReadySession(context.Background(), []string{"apache"}, time.Minute, time.Second)
			errchan <- err
		}()

		assert.Error(t, <-errchan, "session was not ready after 1s")
	})
}

func TestSessionsManager_RequestReadySession(t *testing.T) {

	t.Run("request ready session is ready", func(t *testing.T) {
		manager, store, _ := setupSessionManager(t)
		store.EXPECT().Get(gomock.Any(), gomock.Any()).Return(instance.State{Name: "apache", Status: instance.Ready}, nil).AnyTimes()
		store.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		errchan := make(chan error)
		go func() {
			_, err := manager.RequestReadySession(context.Background(), []string{"apache"}, time.Minute, time.Second)
			errchan <- err
		}()

		assert.NilError(t, <-errchan)
	})
}
