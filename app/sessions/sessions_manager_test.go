package sessions

import (
	"sync"
	"testing"

	"github.com/acouvreur/sablier/app/instance"
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
					{Name: "nginx", Status: instance.Error, Error: "connection timeout"},
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

func createMap(instances []instance.State) (store *sync.Map) {
	store = &sync.Map{}

	for _, v := range instances {
		store.Store(v.Name, InstanceState{
			Instance: &v,
			Error:    nil,
		})
	}

	return
}
