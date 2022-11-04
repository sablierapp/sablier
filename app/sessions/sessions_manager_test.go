package sessions

import (
	"sync"
	"testing"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/sessions/mocks"
	"github.com/stretchr/testify/mock"
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
			provider := mocks.NewProviderMock(tt.stoppedInstances)
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
