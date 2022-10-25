package providers

import (
	"reflect"
	"testing"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers/mocks"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/mock"
)

func TestDockerSwarmProvider_Start(t *testing.T) {
	type fields struct {
		Client *mocks.ServiceAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        instance.State
		serviceList []swarm.Service
		response    types.ServiceUpdateResponse
		wantService swarm.Service
		wantErr     bool
	}{
		{
			name: "scale nginx service to 1 replica",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 0),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     false,
		},
		{
			name: "ambiguous service name",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("STACK1_nginx", 0),
				mocks.ServiceReplicated("STACK2_nginx", 0),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.Error,
				Error:           "ambiguous service names found for \"nginx\" (STACK1_nginx, STACK2_nginx)",
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     true,
		},
		{
			name: "exact match service name",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 0),
				mocks.ServiceReplicated("STACK1_nginx", 0),
				mocks.ServiceReplicated("STACK2_nginx", 0),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     false,
		},
		{
			name: "service match on suffix",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("STACK1_nginx", 0),
				mocks.ServiceReplicated("STACK2_nginx-2", 0),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx (STACK1_nginx)",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantService: mocks.ServiceReplicated("STACK1_nginx", 1),
			wantErr:     false,
		},
		{
			name: "nginx is not a replicated service",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceGlobal("nginx"),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.Error,
				Error:           "swarm service is not in \"replicated\" mode",
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerSwarmProvider{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			tt.fields.Client.On("ServiceUpdate", mock.Anything, tt.wantService.ID, tt.wantService.Meta.Version, tt.wantService.Spec, mock.Anything).Return(tt.response, nil)

			got, err := provider.Start(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerSwarmProvider.Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerSwarmProvider.Start() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerSwarmProvider_Stop(t *testing.T) {
	type fields struct {
		Client *mocks.ServiceAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        instance.State
		serviceList []swarm.Service
		response    types.ServiceUpdateResponse
		wantService swarm.Service
		wantErr     bool
	}{
		{
			name: "scale nginx service to 0 replica",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 1),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantService: mocks.ServiceReplicated("nginx", 0),
			wantErr:     false,
		},
		{
			name: "ambiguous service name",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("STACK1_nginx", 1),
				mocks.ServiceReplicated("STACK2_nginx", 1),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.Error,
				Error:           "ambiguous service names found for \"nginx\" (STACK1_nginx, STACK2_nginx)",
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     true,
		},
		{
			name: "exact match service name",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 1),
				mocks.ServiceReplicated("STACK1_nginx", 1),
				mocks.ServiceReplicated("STACK2_nginx", 1),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantService: mocks.ServiceReplicated("nginx", 0),
			wantErr:     false,
		},
		{
			name: "service match on suffix",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("STACK1_nginx", 1),
				mocks.ServiceReplicated("STACK2_nginx-2", 1),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx (STACK1_nginx)",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantService: mocks.ServiceReplicated("STACK1_nginx", 0),
			wantErr:     false,
		},
		{
			name: "nginx is not a replicated service",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceGlobal("nginx"),
			},
			response: types.ServiceUpdateResponse{
				Warnings: []string{},
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.Error,
				Error:           "swarm service is not in \"replicated\" mode",
			},
			wantService: mocks.ServiceReplicated("nginx", 1),
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerSwarmProvider{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			tt.fields.Client.On("ServiceUpdate", mock.Anything, tt.wantService.ID, tt.wantService.Meta.Version, tt.wantService.Spec, mock.Anything).Return(tt.response, nil)

			got, err := provider.Stop(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerSwarmProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerSwarmProvider.Stop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerSwarmProvider_GetState(t *testing.T) {
	type fields struct {
		Client *mocks.ServiceAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        instance.State
		serviceList []swarm.Service
		wantErr     bool
	}{
		{
			name: "nginx service is ready",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceReplicated("nginx", 1),
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr: false,
		},
		{
			name: "nginx service is not ready",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceNotReadyReplicated("nginx", 1, 0),
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantErr: false,
		},
		{
			name: "nginx service is not ready",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceNotReadyReplicated("STACK_nginx", 1, 0),
			},
			want: instance.State{
				Name:            "nginx (STACK_nginx)",
				CurrentReplicas: 0,
				Status:          instance.NotReady,
			},
			wantErr: false,
		},
		{
			name: "nginx is not a replicated service",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				mocks.ServiceGlobal("nginx"),
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				Status:          instance.Error,
				Error:           "swarm service is not in \"replicated\" mode",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DockerSwarmProvider{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)

			got, err := provider.GetState(tt.args.name)
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
