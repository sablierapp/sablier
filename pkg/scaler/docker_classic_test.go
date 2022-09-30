package scaler

import (
	"context"
	"errors"
	"testing"

	"github.com/acouvreur/sablier/pkg/scaler/mocks"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerClassicScaler_ScaleUp(t *testing.T) {
	type fields struct {
		Client *mocks.ContainerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		containerList []types.Container
		err           error
	}{
		{
			name: "start nginx container",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			err: nil,
		},
		{
			name: "container nginx was not found",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{},
			err:           errors.New("container with name nginx was not found"),
		},
		{
			name: "multiple containers with name nginx were found",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx1"},
				},
				{
					Names: []string{"nginx2"},
				},
			},
			err: errors.New("multiple containers (2) with name nginx were found: [{ [nginx1]    0 [] 0 0 map[]   {} <nil> []} { [nginx2]    0 [] 0 0 map[]   {} <nil> []}]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerClassicScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ContainerList", mock.Anything, mock.Anything).Return(tt.containerList, nil)
			tt.fields.Client.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			err := scaler.ScaleUp(tt.args.name)

			assert.EqualValues(t, tt.err, err)
		})
	}
}

func TestDockerClassicScaler_ScaleDown(t *testing.T) {
	type fields struct {
		Client *mocks.ContainerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		containerList []types.Container
		err           error
	}{
		{
			name: "start nginx container",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			err: nil,
		},
		{
			name: "container nginx was not found",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{},
			err:           errors.New("container with name nginx was not found"),
		},
		{
			name: "multiple containers with name nginx were found",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx1"},
				},
				{
					Names: []string{"nginx2"},
				},
			},
			err: errors.New("multiple containers (2) with name nginx were found: [{ [nginx1]    0 [] 0 0 map[]   {} <nil> []} { [nginx2]    0 [] 0 0 map[]   {} <nil> []}]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerClassicScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ContainerList", mock.Anything, mock.Anything).Return(tt.containerList, nil)
			tt.fields.Client.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			err := scaler.ScaleDown(tt.args.name)

			assert.EqualValues(t, tt.err, err)
		})
	}
}

func TestDockerClassicScaler_IsUp(t *testing.T) {
	type fields struct {
		Client *mocks.ContainerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		containerList []types.Container
		containerSpec types.ContainerJSON
		want          bool
	}{
		{
			name: "nginx container is started without healthcheck",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			containerSpec: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Running: true,
					},
				},
			},
			want: true,
		},
		{
			name: "nginx container is not running without healthcheck",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			containerSpec: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Running: false,
					},
				},
			},
			want: false,
		},
		{
			name: "nginx container is started but not healthy",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			containerSpec: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Running: true,
						Health: &types.Health{
							Status: "starting",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "nginx container is started and healthy",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			containerSpec: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Running: true,
						Health: &types.Health{
							Status: "healthy",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerClassicScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ContainerList", mock.Anything, mock.Anything).Return(tt.containerList, nil)
			tt.fields.Client.On("ContainerInspect", mock.Anything, mock.Anything).Return(tt.containerSpec, nil)

			got := scaler.IsUp(tt.args.name)

			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestDockerClassicScaler_GetContainerByName(t *testing.T) {
	type fields struct {
		Client *mocks.ContainerAPIClientMock
	}
	type args struct {
		name string
		ctx  context.Context
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		containerList []types.Container
		err           error
	}{
		{
			name: "start nginx container",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx"},
				},
			},
			err: nil,
		},
		{
			name: "container nginx was not found",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{},
			err:           errors.New("container with name nginx was not found"),
		},
		{
			name: "multiple containers with name nginx were found",
			fields: fields{
				Client: mocks.NewContainerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			containerList: []types.Container{
				{
					Names: []string{"nginx1"},
				},
				{
					Names: []string{"nginx2"},
				},
			},
			err: errors.New("multiple containers (2) with name nginx were found: [{ [nginx1]    0 [] 0 0 map[]   {} <nil> []} { [nginx2]    0 [] 0 0 map[]   {} <nil> []}]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerClassicScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ContainerList", mock.Anything, mock.Anything).Return(tt.containerList, nil)

			_, err := scaler.GetContainerByName(tt.args.name, tt.args.ctx)

			assert.EqualValues(t, tt.err, err)
		})
	}
}
