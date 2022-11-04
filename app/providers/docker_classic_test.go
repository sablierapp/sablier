package providers

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers/mocks"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/stretchr/testify/mock"
)

func TestDockerClassicProvider_GetState(t *testing.T) {
	type fields struct {
		Client *mocks.DockerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		want          instance.State
		wantErr       bool
		containerSpec types.ContainerJSON
		err           error
	}{
		{
			name: "nginx created container state",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.CreatedContainerSpec("nginx"),
		},
		{
			name: "nginx running container state without healthcheck",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithoutHealthcheckContainerSpec("nginx"),
		},
		{
			name: "nginx running container state with \"starting\" health",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithHealthcheckContainerSpec("nginx", "starting"),
		},
		{
			name: "nginx running container state with \"unhealthy\" health",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container is unhealthy: curl http://localhost failed (1)",
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithHealthcheckContainerSpec("nginx", "unhealthy"),
		},
		{
			name: "nginx running container state with \"healthy\" health",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithHealthcheckContainerSpec("nginx", "healthy"),
		},
		{
			name: "nginx paused container state",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.PausedContainerSpec("nginx"),
		},
		{
			name: "nginx restarting container state",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.RestartingContainerSpec("nginx"),
		},
		{
			name: "nginx removing container state",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.RemovingContainerSpec("nginx"),
		},
		{
			name: "nginx exited container state with status code 0",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.ExitedContainerSpec("nginx", 0),
		},
		{
			name: "nginx exited container state with status code 137",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container exited with code \"137\"",
			},
			wantErr:       false,
			containerSpec: mocks.ExitedContainerSpec("nginx", 137),
		},
		{
			name: "nginx dead container state",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container in \"dead\" state cannot be restarted",
			},
			wantErr:       false,
			containerSpec: mocks.DeadContainerSpec("nginx"),
		},
		{
			name: "container inspect has an error",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container with name \"nginx\" was not found",
			},
			wantErr:       true,
			containerSpec: types.ContainerJSON{},
			err:           fmt.Errorf("container with name \"nginx\" was not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerClassicProvider{
				Client:          tt.fields.Client,
				desiredReplicas: 1,
			}

			tt.fields.Client.On("ContainerInspect", mock.Anything, mock.Anything).Return(tt.containerSpec, tt.err)

			got, err := provider.GetState(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.GetState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerClassicProvider.GetState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerClassicProvider_Stop(t *testing.T) {
	type fields struct {
		Client *mocks.DockerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    instance.State
		wantErr bool
		err     error
	}{
		{
			name: "container stop has an error",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container with name \"nginx\" was not found",
			},
			wantErr: true,
			err:     fmt.Errorf("container with name \"nginx\" was not found"),
		},
		{
			name: "container stop as expected",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: false,
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerClassicProvider{
				Client:          tt.fields.Client,
				desiredReplicas: 1,
			}

			tt.fields.Client.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).Return(tt.err)

			got, err := provider.Stop(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerClassicProvider.Stop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerClassicProvider_Start(t *testing.T) {
	type fields struct {
		Client *mocks.DockerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    instance.State
		wantErr bool
		err     error
	}{
		{
			name: "container start has an error",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container with name \"nginx\" was not found",
			},
			wantErr: true,
			err:     fmt.Errorf("container with name \"nginx\" was not found"),
		},
		{
			name: "container start as expected",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: false,
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerClassicProvider{
				Client:          tt.fields.Client,
				desiredReplicas: 1,
			}

			tt.fields.Client.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(tt.err)

			got, err := provider.Start(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerClassicProvider.Start() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerClassicProvider_NotifyInsanceStopped(t *testing.T) {
	tests := []struct {
		name   string
		want   []string
		events []events.Message
		errors []error
	}{
		{
			name: "container nginx is stopped",
			want: []string{"nginx"},
			events: []events.Message{
				mocks.ContainerStoppedEvent("nginx"),
			},
			errors: []error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerClassicProvider{
				Client:          mocks.NewDockerAPIClientMockWithEvents(tt.events, tt.errors),
				desiredReplicas: 1,
			}

			instanceC := make(chan string)

			provider.NotifyInsanceStopped(context.Background(), instanceC)

			var got []string

			for i := range instanceC {
				got = append(got, i)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NotifyInsanceStopped() = %v, want %v", got, tt.want)
			}
		})
	}
}
