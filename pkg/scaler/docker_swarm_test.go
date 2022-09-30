package scaler

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/acouvreur/sablier/pkg/scaler/mocks"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerSwarmScaler_ScaleUp(t *testing.T) {
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
		serviceList []swarm.Service
		want        swarm.Service
		err         error
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
				{
					ID:   "nginx_service",
					Meta: swarm.Meta{Version: swarm.Version{}},
					Spec: swarm.ServiceSpec{
						Mode: swarm.ServiceMode{
							Replicated: &swarm.ReplicatedService{
								Replicas: &zeroreplicas,
							},
						},
					},
				},
			},
			want: swarm.Service{
				ID:   "nginx_service",
				Meta: swarm.Meta{Version: swarm.Version{}},
				Spec: swarm.ServiceSpec{
					Mode: swarm.ServiceMode{
						Replicated: &swarm.ReplicatedService{
							Replicas: &onereplicas,
						},
					},
				},
			},
			err: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerSwarmScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			tt.fields.Client.On("ServiceUpdate", mock.Anything, tt.want.ID, tt.want.Meta.Version, tt.want.Spec, mock.Anything).Return(types.ServiceUpdateResponse{
				Warnings: []string{},
			}, nil)

			scaler.ScaleUp(tt.args.name)

			tt.fields.Client.AssertExpectations(t)
		})
	}

	t.Run("scale nginx service to 1 replica twice", func(t *testing.T) {
		swarmMock := mocks.NewServiceAPIClientMock()

		serviceList := []swarm.Service{
			{
				ID:   "nginx_service",
				Meta: swarm.Meta{Version: swarm.Version{}},
				Spec: swarm.ServiceSpec{
					Mode: swarm.ServiceMode{
						Replicated: &swarm.ReplicatedService{
							Replicas: &zeroreplicas,
						},
					},
				},
			},
		}

		swarmMock.On("ServiceList", mock.Anything, mock.Anything).Return(serviceList, nil)
		swarmMock.On("ServiceUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(types.ServiceUpdateResponse{
			Warnings: []string{},
		}, nil)

		scaler := &DockerSwarmScaler{
			Client: swarmMock,
		}

		scaler.ScaleUp("nginx")
		scaler.ScaleUp("nginx")

		swarmMock.AssertNumberOfCalls(t, "ServiceUpdate", 1)
	})
}

func TestDockerSwarmScaler_ScaleDown(t *testing.T) {
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
		serviceList []swarm.Service
		want        swarm.Service
		err         error
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
				{
					ID:   "nginx_service",
					Meta: swarm.Meta{Version: swarm.Version{}},
					Spec: swarm.ServiceSpec{
						Mode: swarm.ServiceMode{
							Replicated: &swarm.ReplicatedService{
								Replicas: &onereplicas,
							},
						},
					},
				},
			},
			want: swarm.Service{
				ID:   "nginx_service",
				Meta: swarm.Meta{Version: swarm.Version{}},
				Spec: swarm.ServiceSpec{
					Mode: swarm.ServiceMode{
						Replicated: &swarm.ReplicatedService{
							Replicas: &zeroreplicas,
						},
					},
				},
			},
			err: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerSwarmScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			tt.fields.Client.On("ServiceUpdate", mock.Anything, tt.want.ID, tt.want.Meta.Version, tt.want.Spec, mock.Anything).Return(types.ServiceUpdateResponse{
				Warnings: []string{},
			}, nil)

			scaler.ScaleDown(tt.args.name)

			tt.fields.Client.AssertExpectations(t)
		})
	}
	t.Run("scale nginx service to 0 replica twice", func(t *testing.T) {
		swarmMock := mocks.NewServiceAPIClientMock()

		serviceList := []swarm.Service{
			{
				ID:   "nginx_service",
				Meta: swarm.Meta{Version: swarm.Version{}},
				Spec: swarm.ServiceSpec{
					Mode: swarm.ServiceMode{
						Replicated: &swarm.ReplicatedService{
							Replicas: &onereplicas,
						},
					},
				},
			},
		}

		swarmMock.On("ServiceList", mock.Anything, mock.Anything).Return(serviceList, nil)
		swarmMock.On("ServiceUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(types.ServiceUpdateResponse{
			Warnings: []string{},
		}, nil)

		scaler := &DockerSwarmScaler{
			Client: swarmMock,
		}

		scaler.ScaleDown("nginx")
		scaler.ScaleDown("nginx")

		swarmMock.AssertNumberOfCalls(t, "ServiceUpdate", 1)
	})
}

func TestDockerSwarmScaler_IsUp(t *testing.T) {
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
		serviceList []swarm.Service
		taskList    []swarm.Task
		want        bool
	}{
		{
			name: "service nginx is 0/0",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				{
					ID:   "nginx_service",
					Meta: swarm.Meta{Version: swarm.Version{}},
					Spec: swarm.ServiceSpec{
						Mode: swarm.ServiceMode{
							Replicated: &swarm.ReplicatedService{
								Replicas: &zeroreplicas,
							},
						},
					},
					ServiceStatus: &swarm.ServiceStatus{
						RunningTasks: 0,
						DesiredTasks: 0,
					},
				},
			},
			want: false,
		},
		{
			name: "service nginx is 1/1 since 10 seconds",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				{
					ID:   "nginx_service",
					Meta: swarm.Meta{Version: swarm.Version{}},
					Spec: swarm.ServiceSpec{
						Mode: swarm.ServiceMode{
							Replicated: &swarm.ReplicatedService{
								Replicas: &zeroreplicas,
							},
						},
					},
					ServiceStatus: &swarm.ServiceStatus{
						RunningTasks: 1,
						DesiredTasks: 1,
					},
				},
			},
			taskList: []swarm.Task{
				{
					Status: swarm.TaskStatus{
						Timestamp: time.Now().Add(-10 * time.Second),
					},
				},
			},
			want: true,
		},
		{
			name: "service nginx is 0/1",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				{
					ID:   "nginx_service",
					Meta: swarm.Meta{Version: swarm.Version{}},
					Spec: swarm.ServiceSpec{
						Mode: swarm.ServiceMode{
							Replicated: &swarm.ReplicatedService{
								Replicas: &zeroreplicas,
							},
						},
					},
					ServiceStatus: &swarm.ServiceStatus{
						RunningTasks: 0,
						DesiredTasks: 1,
					},
				},
			},
			taskList: []swarm.Task{},
			want:     false,
		},
		{
			name: "service nginx is 1/1 since 2 seconds",
			fields: fields{
				Client: mocks.NewServiceAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			serviceList: []swarm.Service{
				{
					ID:   "nginx_service",
					Meta: swarm.Meta{Version: swarm.Version{}},
					Spec: swarm.ServiceSpec{
						Mode: swarm.ServiceMode{
							Replicated: &swarm.ReplicatedService{
								Replicas: &zeroreplicas,
							},
						},
					},
					ServiceStatus: &swarm.ServiceStatus{
						RunningTasks: 0,
						DesiredTasks: 1,
					},
				},
			},
			taskList: []swarm.Task{
				{
					Status: swarm.TaskStatus{
						Timestamp: time.Now().Add(-2 * time.Second),
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerSwarmScaler{
				Client: tt.fields.Client,
			}

			tt.fields.Client.On("ServiceList", mock.Anything, mock.Anything).Return(tt.serviceList, nil)
			tt.fields.Client.On("TaskList", mock.Anything, mock.Anything).Return(tt.taskList, nil)

			got := scaler.IsUp(tt.args.name)

			assert.EqualValues(t, tt.want, got)
			//tt.fields.Client.AssertExpectations(t)
		})
	}
}

func TestDockerSwarmScaler_GetServiceByName(t *testing.T) {
	type fields struct {
		Client client.ServiceAPIClient
	}
	type args struct {
		name string
		ctx  context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *swarm.Service
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &DockerSwarmScaler{
				Client: tt.fields.Client,
			}
			got, err := scaler.GetServiceByName(tt.args.name, tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerSwarmScaler.GetServiceByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerSwarmScaler.GetServiceByName() = %v, want %v", got, tt.want)
			}
		})
	}
}
