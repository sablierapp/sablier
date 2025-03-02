package dockerswarm

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/mocks"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/swarm"
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

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NotifyInstanceStopped() = %v, want %v", got, tt.want)
			}
		})
	}
}
