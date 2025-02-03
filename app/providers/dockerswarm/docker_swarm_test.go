package dockerswarm

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/neilotoole/slogt"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/swarm"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/providers/mocks"
	"github.com/stretchr/testify/mock"
)

func setupProvider(t *testing.T, client client.APIClient) *DockerSwarmProvider {
	t.Helper()
	return &DockerSwarmProvider{
		Client:          client,
		desiredReplicas: 1,
		l:               slogt.New(t),
	}
}

func TestDockerSwarmProvider_Start(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name        string
		args        args
		serviceList []swarm.Service
		response    swarm.ServiceUpdateResponse
		wantService swarm.Service
		wantErr     bool
	}{
		{
			name: "scale nginx service to 1 replica",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 0),
			},
			response: swarm.ServiceUpdateResponse{
				Warnings: []string{},
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     false,
		},
		{
			name: "exact match service name",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 0),
				mocks.ServiceReplicated("STACK1_nginx", 0),
				mocks.ServiceReplicated("STACK2_nginx", 0),
			},
			response: swarm.ServiceUpdateResponse{
				Warnings: []string{},
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     false,
		},
		{
			name: "nginx is not a replicated service",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceGlobal("nginx"),
			},
			response: swarm.ServiceUpdateResponse{
				Warnings: []string{},
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientMock := mocks.NewDockerAPIClientMock()
			provider := setupProvider(t, clientMock)

			clientMock.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			clientMock.On("ServiceUpdate", mock.Anything, tt.wantService.ID, tt.wantService.Meta.Version, tt.wantService.Spec, mock.Anything).Return(tt.response, nil)

			err := provider.Start(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerSwarmProvider.Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDockerSwarmProvider_Stop(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name        string
		args        args
		serviceList []swarm.Service
		response    swarm.ServiceUpdateResponse
		wantService swarm.Service
		wantErr     bool
	}{
		{
			name: "scale nginx service to 0 replica",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 1),
			},
			response: swarm.ServiceUpdateResponse{
				Warnings: []string{},
			},
			wantService: mocks.ServiceReplicated("nginx", 0),
			wantErr:     false,
		},
		{
			name: "exact match service name",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 1),
				mocks.ServiceReplicated("STACK1_nginx", 1),
				mocks.ServiceReplicated("STACK2_nginx", 1),
			},
			response: swarm.ServiceUpdateResponse{
				Warnings: []string{},
			},
			wantService: mocks.ServiceReplicated("nginx", 0),
			wantErr:     false,
		},
		{
			name: "nginx is not a replicated service",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceGlobal("nginx"),
			},
			response: swarm.ServiceUpdateResponse{
				Warnings: []string{},
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientMock := mocks.NewDockerAPIClientMock()
			provider := setupProvider(t, clientMock)

			clientMock.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			clientMock.On("ServiceUpdate", mock.Anything, tt.wantService.ID, tt.wantService.Meta.Version, tt.wantService.Spec, mock.Anything).Return(tt.response, nil)

			err := provider.Stop(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerSwarmProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDockerSwarmProvider_GetState(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name        string
		args        args
		want        instance.State
		serviceList []swarm.Service
		wantErr     bool
	}{
		{
			name: "nginx service is ready",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 1),
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr: false,
		},
		{
			name: "nginx service is not ready",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceNotReadyReplicated("nginx", 1, 0),
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: false,
		},
		{
			name: "nginx is not a replicated service",
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceGlobal("nginx"),
			},
			want:    instance.State{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientMock := mocks.NewDockerAPIClientMock()
			provider := setupProvider(t, clientMock)

			clientMock.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)

			got, err := provider.GetState(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerSwarmProvider.GetState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerSwarmProvider.GetState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerSwarmProvider_NotifyInstanceStopped(t *testing.T) {
	tests := []struct {
		name   string
		want   []string
		events []events.Message
		errors []error
	}{
		{
			name: "service nginx is scaled to 0",
			want: []string{"nginx"},
			events: []events.Message{
				mocks.ServiceScaledEvent("nginx", "1", "0"),
			},
			errors: []error{},
		}, {
			name: "service nginx is scaled to 0",
			want: []string{"nginx"},
			events: []events.Message{
				mocks.ServiceRemovedEvent("nginx"),
			},
			errors: []error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupProvider(t, mocks.NewDockerAPIClientMockWithEvents(tt.events, tt.errors))

			instanceC := make(chan string)

			ctx, cancel := context.WithCancel(context.Background())
			provider.NotifyInstanceStopped(ctx, instanceC)

			var got []string

			got = append(got, <-instanceC)
			cancel()
			close(instanceC)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NotifyInstanceStopped() = %v, want %v", got, tt.want)
			}
		})
	}
}
