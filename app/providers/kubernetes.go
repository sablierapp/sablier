package providers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/acouvreur/sablier/app/instance"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Delimiter is used to split name into kind,namespace,name,replicacount
const Delimiter = "_"

type Config struct {
	OriginalName string
	Kind         string // deployment or statefulset
	Namespace    string
	Name         string
	Replicas     int32
}

type Workload interface {
	GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

func convertName(name string) (*Config, error) {
	// name format kind_namespace_name_replicas
	s := strings.Split(name, Delimiter)
	if len(s) < 4 {
		return nil, errors.New("invalid name should be: kind" + Delimiter + "namespace" + Delimiter + "name" + Delimiter + "replicas")
	}
	replicas, err := strconv.Atoi(s[3])
	if err != nil {
		return nil, err
	}

	return &Config{
		OriginalName: name,
		Kind:         s[0],
		Namespace:    s[1],
		Name:         s[2],
		Replicas:     int32(replicas),
	}, nil
}

type KubernetesProvider struct {
	Client kubernetes.Interface
}

func NewKubernetesProvider() (*KubernetesProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &KubernetesProvider{
		Client: client,
	}, nil
}

func (provider *KubernetesProvider) Start(name string) (instance.State, error) {
	config, err := convertName(name)
	if err != nil {
		return instance.UnrecoverableInstanceState(name, err.Error(), int(config.Replicas))
	}

	return provider.scale(config, config.Replicas)
}

func (provider *KubernetesProvider) Stop(name string) (instance.State, error) {
	config, err := convertName(name)
	if err != nil {
		return instance.UnrecoverableInstanceState(name, err.Error(), int(config.Replicas))
	}

	return provider.scale(config, 0)

}

func (provider *KubernetesProvider) scale(config *Config, replicas int32) (instance.State, error) {
	ctx := context.Background()

	var workload Workload

	switch config.Kind {
	case "deployment":
		workload = provider.Client.AppsV1().Deployments(config.Namespace)
	case "statefulset":
		workload = provider.Client.AppsV1().StatefulSets(config.Namespace)
	default:
		return instance.UnrecoverableInstanceState(config.OriginalName, fmt.Sprintf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", config.Kind), int(config.Replicas))
	}

	s, err := workload.GetScale(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return instance.ErrorInstanceState(config.OriginalName, err, int(config.Replicas))
	}

	s.Spec.Replicas = replicas
	_, err = workload.UpdateScale(ctx, config.Name, s, metav1.UpdateOptions{})

	if err != nil {
		return instance.ErrorInstanceState(config.OriginalName, err, int(config.Replicas))
	}

	return instance.NotReadyInstanceState(config.OriginalName, 0, int(config.Replicas))
}

func (provider *KubernetesProvider) GetState(name string) (instance.State, error) {
	config, err := convertName(name)
	if err != nil {
		return instance.UnrecoverableInstanceState(name, err.Error(), int(config.Replicas))
	}

	switch config.Kind {
	case "deployment":
		return provider.getDeploymentState(config)
	case "statefulset":
		return provider.getStatefulsetState(config)
	default:
		return instance.UnrecoverableInstanceState(config.OriginalName, fmt.Sprintf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", config.Kind), int(config.Replicas))
	}
}

func (provider *KubernetesProvider) getDeploymentState(config *Config) (instance.State, error) {
	ctx := context.Background()

	d, err := provider.Client.AppsV1().Deployments(config.Namespace).
		Get(ctx, config.Name, metav1.GetOptions{})

	if err != nil {
		return instance.ErrorInstanceState(config.OriginalName, err, int(config.Replicas))
	}

	if *d.Spec.Replicas == d.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.OriginalName, int(config.Replicas))
	}

	return instance.NotReadyInstanceState(config.OriginalName, int(d.Status.ReadyReplicas), int(config.Replicas))
}

func (provider *KubernetesProvider) getStatefulsetState(config *Config) (instance.State, error) {
	ctx := context.Background()

	ss, err := provider.Client.AppsV1().StatefulSets(config.Namespace).
		Get(ctx, config.Name, metav1.GetOptions{})

	if err != nil {
		return instance.ErrorInstanceState(config.OriginalName, err, int(config.Replicas))
	}

	if *ss.Spec.Replicas == ss.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.OriginalName, int(config.Replicas))
	}

	return instance.NotReadyInstanceState(config.OriginalName, int(ss.Status.ReadyReplicas), int(config.Replicas))
}

func (provider *KubernetesProvider) NotifyInsanceStopped(ctx context.Context, instance chan string) {
}
