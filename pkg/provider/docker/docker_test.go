package docker_test

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
	"reflect"
	"testing"
	"time"

	"github.com/sablierapp/sablier/app/instance"
)

/*
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
			wantErr: false,
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupProvider(t, tt.fields.Client)

			tt.fields.Client.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).Return(tt.err)

			err := provider.Stop(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
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
			wantErr: false,
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupProvider(t, tt.fields.Client)

			tt.fields.Client.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(tt.err)

			err := provider.Start(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDockerClassicProvider_NotifyInstanceStopped(t *testing.T) {
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
			provider := setupProvider(t, mocks.NewDockerAPIClientMockWithEvents(tt.events, tt.errors))

			instanceC := make(chan string, 1)

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
*/
