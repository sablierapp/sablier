package mocks

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
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

type KubernetesAPIClientMock struct {
	mockv1 AppsV1InterfaceMock

	kubernetes.Clientset
}

type AppsV1InterfaceMock struct {
	deployments  *DeploymentMock
	statefulsets *StatefulSetsMock

	v1.AppsV1Interface
}

type DeploymentMock struct {
	v1.DeploymentInterface
	mock.Mock
}

func (d DeploymentMock) Get(ctx context.Context, workloadName string, options metav1.GetOptions) (*appsv1.Deployment, error) {
	args := d.Mock.Called(ctx, workloadName, options)
	if args.Get(0) != nil {
		return args.Get(0).(*appsv1.Deployment), args.Error(1)
	}
	return nil, args.Error(1)
}

func (d DeploymentMock) GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error) {
	args := d.Mock.Called(ctx, workloadName, options)
	return args.Get(0).(*autoscalingv1.Scale), args.Error(1)
}

func (d DeploymentMock) UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error) {
	args := d.Mock.Called(ctx, workloadName, scale, opts)
	if args.Get(0) != nil {
		return args.Get(0).(*autoscalingv1.Scale), args.Error(1)
	}
	return nil, args.Error(1)
}

func (api AppsV1InterfaceMock) Deployments(namespace string) v1.DeploymentInterface {
	return api.deployments
}

type StatefulSetsMock struct {
	v1.StatefulSetInterface
	mock.Mock
}

func (ss StatefulSetsMock) Get(ctx context.Context, workloadName string, options metav1.GetOptions) (*appsv1.StatefulSet, error) {
	args := ss.Mock.Called(ctx, workloadName, options)
	if args.Get(0) != nil {
		return args.Get(0).(*appsv1.StatefulSet), args.Error(1)
	}
	return nil, args.Error(1)
}

func (ss StatefulSetsMock) GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error) {
	args := ss.Mock.Called(ctx, workloadName, options)
	return args.Get(0).(*autoscalingv1.Scale), args.Error(1)
}

func (ss StatefulSetsMock) UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error) {
	args := ss.Mock.Called(ctx, workloadName, scale, opts)
	if args.Get(0) != nil {
		return args.Get(0).(*autoscalingv1.Scale), args.Error(1)
	}
	return nil, args.Error(1)
}

func (api AppsV1InterfaceMock) StatefulSets(namespace string) v1.StatefulSetInterface {
	return api.statefulsets
}

func (c *KubernetesAPIClientMock) AppsV1() v1.AppsV1Interface {
	return c.mockv1
}

func NewKubernetesAPIClientMock(deployments *DeploymentMock, statefulsets *StatefulSetsMock) *KubernetesAPIClientMock {
	return &KubernetesAPIClientMock{
		mockv1: AppsV1InterfaceMock{
			deployments:  deployments,
			statefulsets: statefulsets,
		},
	}
}

func V1Scale(replicas int) *autoscalingv1.Scale {
	return &autoscalingv1.Scale{
		Spec: autoscalingv1.ScaleSpec{
			Replicas: int32(replicas),
		},
	}
}

func V1Deployment(replicas int, readyReplicas int) *appsv1.Deployment {
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: make(int32(replicas)),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: int32(readyReplicas),
		},
	}
}

func V1StatefulSet(replicas int, readyReplicas int) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: make(int32(replicas)),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: int32(readyReplicas),
		},
	}
}

func make(val int32) *int32 {
	return &val
}
