package mocks

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/mock"
)

type ContainerAPIClientMock struct {
	client.ContainerAPIClient
	mock.Mock
}

func NewContainerAPIClientMock() *ContainerAPIClientMock {
	return &ContainerAPIClientMock{}
}

func (client *ContainerAPIClientMock) ContainerStart(ctx context.Context, container string, options types.ContainerStartOptions) error {
	args := client.Mock.Called(ctx, container, options)
	return args.Error(0)
}

func (client *ContainerAPIClientMock) ContainerStop(ctx context.Context, container string, timeout *time.Duration) error {
	args := client.Mock.Called(ctx, container, timeout)
	return args.Error(0)
}

func (client *ContainerAPIClientMock) ContainerInspect(ctx context.Context, container string) (types.ContainerJSON, error) {
	args := client.Mock.Called(ctx, container)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func CreatedContainerSpec(name string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Running: false,
				Status:  "created",
			},
		},
	}
}

func RunningWithoutHealthcheckContainerSpec(name string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Running: true,
				Status:  "running",
			},
		},
	}
}

// Status can be "starting", "healthy" or "unhealthy"
func RunningWithHealthcheckContainerSpec(name string, status string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Running: true,
				Status:  "running",
				Health: &types.Health{
					Status: status,
					Log: []*types.HealthcheckResult{
						{
							Start:    time.Now().Add(-5 * time.Second),
							End:      time.Now(),
							Output:   "curl http://localhost failed",
							ExitCode: 1,
						},
					},
				},
			},
		},
	}
}

func PausedContainerSpec(name string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Paused: true,
				Status: "paused",
			},
		},
	}
}

func RestartingContainerSpec(name string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Restarting: true,
				Status:     "restarting",
			},
		},
	}
}

func RemovingContainerSpec(name string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Status: "removing",
			},
		},
	}
}

func ExitedContainerSpec(name string, exitCode int) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				ExitCode: exitCode,
				Status:   "exited",
			},
		},
	}
}

func DeadContainerSpec(name string) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: name,
			ID:   name,
			State: &types.ContainerState{
				Dead:   true,
				Status: "dead",
			},
		},
	}
}

type ServiceAPIClientMock struct {
	client.ServiceAPIClient
	mock.Mock
}

func NewServiceAPIClientMock() *ServiceAPIClientMock {
	return &ServiceAPIClientMock{}
}

func (client *ServiceAPIClientMock) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error) {
	args := client.Mock.Called(ctx, serviceID, version, service, options)
	return args.Get(0).(types.ServiceUpdateResponse), args.Error(1)
}

func (client *ServiceAPIClientMock) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	args := client.Mock.Called(ctx, options)
	return args.Get(0).([]swarm.Service), args.Error(1)
}

func ServiceReplicated(name string, replicas uint64) swarm.Service {
	return swarm.Service{
		ID:   name,
		Meta: swarm.Meta{Version: swarm.Version{}},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: name,
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: &replicas,
				},
			},
		},
		ServiceStatus: &swarm.ServiceStatus{
			RunningTasks: replicas,
			DesiredTasks: replicas,
		},
	}
}

func ServiceNotReadyReplicated(name string, runningTasks uint64, desiredTasks uint64) swarm.Service {
	return swarm.Service{
		ID:   name,
		Meta: swarm.Meta{Version: swarm.Version{}},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: name,
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: &desiredTasks,
				},
			},
		},
		ServiceStatus: &swarm.ServiceStatus{
			RunningTasks: runningTasks,
			DesiredTasks: desiredTasks,
		},
	}
}

func ServiceGlobal(name string) swarm.Service {
	return swarm.Service{
		ID:   name,
		Meta: swarm.Meta{Version: swarm.Version{}},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: name,
			},
			Mode: swarm.ServiceMode{
				Global: &swarm.GlobalService{},
			},
		},
	}
}
