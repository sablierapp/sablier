package kubernetes

import (
	"context"
	"github.com/neilotoole/slogt"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"testing"

	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/providers/mocks"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setupProvider(t *testing.T, client kubernetes.Interface) *KubernetesProvider {
	t.Helper()
	return &KubernetesProvider{
		Client:    client,
		delimiter: "_",
		l:         slogt.New(t),
	}
}

func TestKubernetesProvider_Start(t *testing.T) {
	type data struct {
		name   string
		get    *autoscalingv1.Scale
		update *autoscalingv1.Scale
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    instance.State
		data    data
		wantErr bool
	}{
		{
			name: "scale nginx deployment to 2 replicas",
			args: args{
				name: "deployment_default_nginx_2",
			},
			data: data{
				name:   "nginx",
				get:    mocks.V1Scale(0),
				update: mocks.V1Scale(2),
			},
			wantErr: false,
		},
		{
			name: "scale nginx statefulset to 2 replicas",
			args: args{
				name: "statefulset_default_nginx_2",
			},
			data: data{
				name:   "nginx",
				get:    mocks.V1Scale(0),
				update: mocks.V1Scale(2),
			},
			wantErr: false,
		},
		{
			name: "scale unsupported kind",
			args: args{
				name: "gateway_default_nginx_2",
			},
			data: data{
				name:   "nginx",
				get:    mocks.V1Scale(0),
				update: mocks.V1Scale(0),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploymentAPI := mocks.DeploymentMock{}
			statefulsetAPI := mocks.StatefulSetsMock{}
			provider := setupProvider(t, mocks.NewKubernetesAPIClientMock(&deploymentAPI, &statefulsetAPI))

			deploymentAPI.On("GetScale", mock.Anything, tt.data.name, metav1.GetOptions{}).Return(tt.data.get, nil)
			deploymentAPI.On("UpdateScale", mock.Anything, tt.data.name, tt.data.update, metav1.UpdateOptions{}).Return(nil, nil)

			statefulsetAPI.On("GetScale", mock.Anything, tt.data.name, metav1.GetOptions{}).Return(tt.data.get, nil)
			statefulsetAPI.On("UpdateScale", mock.Anything, tt.data.name, tt.data.update, metav1.UpdateOptions{}).Return(nil, nil)

			err := provider.Start(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("KubernetesProvider.Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestKubernetesProvider_Stop(t *testing.T) {
	type data struct {
		name   string
		get    *autoscalingv1.Scale
		update *autoscalingv1.Scale
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    instance.State
		data    data
		wantErr bool
	}{
		{
			name: "scale nginx deployment to 2 replicas",
			args: args{
				name: "deployment_default_nginx_2",
			},
			data: data{
				name:   "nginx",
				get:    mocks.V1Scale(2),
				update: mocks.V1Scale(0),
			},
			wantErr: false,
		},
		{
			name: "scale nginx statefulset to 2 replicas",
			args: args{
				name: "statefulset_default_nginx_2",
			},
			data: data{
				name:   "nginx",
				get:    mocks.V1Scale(2),
				update: mocks.V1Scale(0),
			},
			wantErr: false,
		},
		{
			name: "scale unsupported kind",
			args: args{
				name: "gateway_default_nginx_2",
			},
			data: data{
				name:   "nginx",
				get:    mocks.V1Scale(0),
				update: mocks.V1Scale(0),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploymentAPI := mocks.DeploymentMock{}
			statefulsetAPI := mocks.StatefulSetsMock{}
			provider := setupProvider(t, mocks.NewKubernetesAPIClientMock(&deploymentAPI, &statefulsetAPI))

			deploymentAPI.On("GetScale", mock.Anything, tt.data.name, metav1.GetOptions{}).Return(tt.data.get, nil)
			deploymentAPI.On("UpdateScale", mock.Anything, tt.data.name, tt.data.update, metav1.UpdateOptions{}).Return(nil, nil)

			statefulsetAPI.On("GetScale", mock.Anything, tt.data.name, metav1.GetOptions{}).Return(tt.data.get, nil)
			statefulsetAPI.On("UpdateScale", mock.Anything, tt.data.name, tt.data.update, metav1.UpdateOptions{}).Return(nil, nil)

			err := provider.Stop(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("KubernetesProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestKubernetesProvider_GetState(t *testing.T) {
	type data struct {
		name           string
		getDeployment  *v1.Deployment
		getStatefulSet *v1.StatefulSet
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    instance.State
		data    data
		wantErr bool
	}{
		{
			name: "ready nginx deployment with 2 ready replicas",
			args: args{
				name: "deployment_default_nginx_2",
			},
			want: instance.State{
				Name:            "deployment_default_nginx_2",
				CurrentReplicas: 2,
				DesiredReplicas: 2,
				Status:          instance.Ready,
			},
			data: data{
				name:          "nginx",
				getDeployment: mocks.V1Deployment(2, 2),
			},
			wantErr: false,
		},
		{
			name: "not ready nginx deployment with 1 ready replica out of 2",
			args: args{
				name: "deployment_default_nginx_2",
			},
			want: instance.State{
				Name:            "deployment_default_nginx_2",
				CurrentReplicas: 1,
				DesiredReplicas: 2,
				Status:          instance.NotReady,
			},
			data: data{
				name:          "nginx",
				getDeployment: mocks.V1Deployment(2, 1),
			},
			wantErr: false,
		},
		{
			name: "ready nginx statefulset to 2 replicas",
			args: args{
				name: "statefulset_default_nginx_2",
			},
			want: instance.State{
				Name:            "statefulset_default_nginx_2",
				CurrentReplicas: 2,
				DesiredReplicas: 2,
				Status:          instance.Ready,
			},
			data: data{
				name:           "nginx",
				getStatefulSet: mocks.V1StatefulSet(2, 2),
			},
			wantErr: false,
		},
		{
			name: "not ready nginx statefulset to 1 ready replica out of 2",
			args: args{
				name: "statefulset_default_nginx_2",
			},
			want: instance.State{
				Name:            "statefulset_default_nginx_2",
				CurrentReplicas: 1,
				DesiredReplicas: 2,
				Status:          instance.NotReady,
			},
			data: data{
				name:           "nginx",
				getStatefulSet: mocks.V1StatefulSet(2, 1),
			},
			wantErr: false,
		},
		{
			name: "scale unsupported kind",
			args: args{
				name: "gateway_default_nginx_2",
			},
			want: instance.State{},
			data: data{
				name: "nginx",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploymentAPI := mocks.DeploymentMock{}
			statefulsetAPI := mocks.StatefulSetsMock{}
			provider := setupProvider(t, mocks.NewKubernetesAPIClientMock(&deploymentAPI, &statefulsetAPI))

			deploymentAPI.On("Get", mock.Anything, tt.data.name, metav1.GetOptions{}).Return(tt.data.getDeployment, nil)
			statefulsetAPI.On("Get", mock.Anything, tt.data.name, metav1.GetOptions{}).Return(tt.data.getStatefulSet, nil)

			got, err := provider.GetState(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("KubernetesProvider.GetState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KubernetesProvider.GetState() = %v, want %v", got, tt.want)
			}
		})
	}
}
