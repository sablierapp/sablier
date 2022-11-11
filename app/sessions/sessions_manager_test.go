package sessions

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/sessions/mocks"
	"github.com/stretchr/testify/mock"
	"gotest.tools/v3/assert"
)

func TestSessionState_IsReady(t *testing.T) {
	type fields struct {
		Instances *sync.Map
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
				Instances: createMap([]*instance.State{
					{Name: "nginx", Status: instance.Ready},
					{Name: "apache", Status: instance.Ready},
				}),
			},
			want: true,
		},
		{
			name: "one instance is not ready",
			fields: fields{
				Instances: createMap([]*instance.State{
					{Name: "nginx", Status: instance.Ready},
					{Name: "apache", Status: instance.NotReady},
				}),
			},
			want: false,
		},
		{
			name: "no instances specified",
			fields: fields{
				Instances: createMap([]*instance.State{}),
			},
			want: true,
		},
		{
			name: "one instance has an error",
			fields: fields{
				Instances: createMap([]*instance.State{
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

func createMap(instances []*instance.State) (store *sync.Map) {
	store = &sync.Map{}

	for _, v := range instances {
		store.Store(v.Name, InstanceState{
			Instance: v,
			Error:    nil,
		})
	}

	return
}

func TestNewSessionsManagerEvents(t *testing.T) {
	tests := []struct {
		name             string
		stoppedInstances []string
	}{
		{
			name:             "when nginx is stopped it is removed from the store",
			stoppedInstances: []string{"nginx"},
		},
		{
			name:             "when nginx, apache and whoami is stopped it is removed from the store",
			stoppedInstances: []string{"nginx", "apache", "whoami"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := mocks.NewProviderMockWithStoppedInstancesEvents(tt.stoppedInstances)
			provider.Add(1)

			kv := mocks.NewKVMock()
			kv.Add(len(tt.stoppedInstances))
			kv.Mock.On("Delete", mock.AnythingOfType("string")).Return()

			NewSessionsManager(kv, provider)

			// The provider watches notifications from a Goroutine, must wait
			provider.Wait()
			// The key is deleted inside a Goroutine by the session manager, must wait
			kv.Wait()

			for _, instance := range tt.stoppedInstances {
				kv.AssertCalled(t, "Delete", instance)
			}
		})
	}
}

func TestSessionsManager_RequestReadySessionCancelledByUser(t *testing.T) {

	t.Run("request ready session is cancelled by user", func(t *testing.T) {
		kvmock := mocks.NewKVMock()
		kvmock.On("Get", mock.Anything).Return(instance.State{Name: "apache", Status: instance.NotReady}, true)

		providermock := mocks.NewProviderMock()
		providermock.On("GetState", mock.Anything).Return(instance.State{Name: "apache", Status: instance.NotReady}, nil)

		s := &SessionsManager{
			store:    kvmock,
			provider: providermock,
		}

		ctx, cancel := context.WithCancel(context.Background())

		errchan := make(chan error)
		go func() {
			_, err := s.RequestReadySession(ctx, []string{"nginx", "whoami"}, time.Minute, time.Minute)
			errchan <- err
		}()

		// Cancel the call
		cancel()

		assert.Error(t, <-errchan, "request cancelled by user")
	})
}

func TestSessionsManager_RequestReadySessionCancelledByTimeout(t *testing.T) {

	t.Run("request ready session is cancelled by timeout", func(t *testing.T) {
		kvmock := mocks.NewKVMock()
		kvmock.On("Get", mock.Anything).Return(instance.State{Name: "apache", Status: instance.NotReady}, true)

		providermock := mocks.NewProviderMock()
		providermock.On("GetState", mock.Anything).Return(instance.State{Name: "apache", Status: instance.NotReady}, nil)

		s := &SessionsManager{
			store:    kvmock,
			provider: providermock,
		}

		errchan := make(chan error)
		go func() {
			_, err := s.RequestReadySession(context.Background(), []string{"nginx", "whoami"}, time.Minute, time.Second)
			errchan <- err
		}()

		assert.Error(t, <-errchan, "session was not ready after 1s")
	})
}

func TestSessionsManager_RequestReadySession(t *testing.T) {

	t.Run("request ready session is ready", func(t *testing.T) {
		kvmock := mocks.NewKVMock()
		kvmock.On("Get", mock.Anything).Return(instance.State{Name: "apache", Status: instance.Ready}, true)

		providermock := mocks.NewProviderMock()

		s := &SessionsManager{
			store:    kvmock,
			provider: providermock,
		}

		errchan := make(chan error)
		go func() {
			_, err := s.RequestReadySession(context.Background(), []string{"nginx", "whoami"}, time.Minute, time.Second)
			errchan <- err
		}()

		assert.NilError(t, <-errchan)
	})
}
