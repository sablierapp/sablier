package mocks

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/mock"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

// DockerAPIClientMock is a mock for the Docker API client
//
// Deprecated: tests should use a testcontainer
type DockerAPIClientMock struct {
	// Will be sent through events
	messages []events.Message
	errors   []error

	client.APIClient
	mock.Mock
}

func NewDockerAPIClientMock() *DockerAPIClientMock {
	return &DockerAPIClientMock{}
}

func NewDockerAPIClientMockWithEvents(messages []events.Message, errors []error) *DockerAPIClientMock {
	return &DockerAPIClientMock{
		messages: messages,
		errors:   errors,
	}
}

func (client *DockerAPIClientMock) ContainerStart(ctx context.Context, container string, options container.StartOptions) error {
	args := client.Mock.Called(ctx, container, options)
	return args.Error(0)
}

func (client *DockerAPIClientMock) ContainerStop(ctx context.Context, container string, options container.StopOptions) error {
	args := client.Mock.Called(ctx, container, options)
	return args.Error(0)
}

func (client *DockerAPIClientMock) ContainerInspect(ctx context.Context, container string) (types.ContainerJSON, error) {
	args := client.Mock.Called(ctx, container)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func (client *DockerAPIClientMock) Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error) {
	// client.Mock.Called(ctx, options)
	evnts := make(chan events.Message)
	errors := make(chan error)
	go func() {
		defer close(evnts)
		defer close(errors)
		for i := 0; i < len(client.messages); i++ {
			evnts <- client.messages[i]
		}
		errors <- io.EOF
	}()
	return evnts, errors
}

func (client *DockerAPIClientMock) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (swarm.ServiceUpdateResponse, error) {
	args := client.Mock.Called(ctx, serviceID, version, service, options)
	return args.Get(0).(swarm.ServiceUpdateResponse), args.Error(1)
}

func (client *DockerAPIClientMock) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
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

func ServiceScaledEvent(name string, oldReplicas string, newReplicas string) events.Message {
	return events.Message{
		Scope:  "swarm",
		Action: "update",
		Type:   "service",
		Actor: events.Actor{
			ID: "randomid",
			Attributes: map[string]string{
				"name":         name,
				"replicas.new": newReplicas,
				"replicas.old": oldReplicas,
			},
		},
	}
}

func ServiceRemovedEvent(name string) events.Message {
	return events.Message{
		Scope:  "swarm",
		Action: "remove",
		Type:   "service",
		Actor: events.Actor{
			ID: "randomid",
			Attributes: map[string]string{
				"name": name,
			},
		},
	}
}

type KubernetesAPIClientMock struct {
	mockv1 AppsV1InterfaceMock

	kubernetes.Clientset
}

// AppsV1InterfaceMock is a mock
//
// Deprecated: tests should use a testcontainer
type AppsV1InterfaceMock struct {
	deployments  *DeploymentMock
	statefulsets *StatefulSetsMock

	v1.AppsV1Interface
}

// DeploymentMock is a mock
//
// Deprecated: tests should use a testcontainer
type DeploymentMock struct {
	v1.DeploymentInterface
	mock.Mock
}

func (d *DeploymentMock) Get(ctx context.Context, workloadName string, options metav1.GetOptions) (*appsv1.Deployment, error) {
	args := d.Mock.Called(ctx, workloadName, options)
	if args.Get(0) != nil {
		return args.Get(0).(*appsv1.Deployment), args.Error(1)
	}
	return nil, args.Error(1)
}

func (d *DeploymentMock) GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error) {
	args := d.Mock.Called(ctx, workloadName, options)
	return args.Get(0).(*autoscalingv1.Scale), args.Error(1)
}

func (d *DeploymentMock) UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error) {
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

func (ss *StatefulSetsMock) Get(ctx context.Context, workloadName string, options metav1.GetOptions) (*appsv1.StatefulSet, error) {
	args := ss.Mock.Called(ctx, workloadName, options)
	if args.Get(0) != nil {
		return args.Get(0).(*appsv1.StatefulSet), args.Error(1)
	}
	return nil, args.Error(1)
}

func (ss *StatefulSetsMock) GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error) {
	args := ss.Mock.Called(ctx, workloadName, options)
	return args.Get(0).(*autoscalingv1.Scale), args.Error(1)
}

func (ss *StatefulSetsMock) UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error) {
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
			Replicas: makeP(int32(replicas)),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: int32(readyReplicas),
		},
	}
}

func V1StatefulSet(replicas int, readyReplicas int) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: makeP(int32(replicas)),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: int32(readyReplicas),
		},
	}
}

func makeP(val int32) *int32 {
	return &val
}
