package providers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"

	"github.com/acouvreur/sablier/app/instance"
	providerConfig "github.com/acouvreur/sablier/config"
	log "github.com/sirupsen/logrus"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

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

func (provider *KubernetesProvider) convertName(name string) (*Config, error) {
	s := strings.Split(name, provider.delimiter)
	if len(s) < 4 {
		return nil, errors.New("invalid name should be: kind" + provider.delimiter + "namespace" + provider.delimiter + "name" + provider.delimiter + "replicas")
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

func (provider *KubernetesProvider) convertStatefulset(ss *appsv1.StatefulSet, replicas int32) string {
	return fmt.Sprintf("statefulset%s%s%s%s%s%d", provider.delimiter, ss.Namespace, provider.delimiter, ss.Name, provider.delimiter, replicas)
}

func (provider *KubernetesProvider) convertDeployment(d *appsv1.Deployment, replicas int32) string {
	return fmt.Sprintf("deployment%s%s%s%s%s%d", provider.delimiter, d.Namespace, provider.delimiter, d.Name, provider.delimiter, replicas)

}

type KubernetesProvider struct {
	Client    kubernetes.Interface
	delimiter string
}

func NewKubernetesProvider(providerConfig providerConfig.Kubernetes) (*KubernetesProvider, error) {
	kubeclientConfig, err := rest.InClusterConfig()

	kubeclientConfig.QPS = providerConfig.QPS
	kubeclientConfig.Burst = providerConfig.Burst

	log.Debug(fmt.Sprintf("Provider configuration:  QPS=%v, Burst=%v", kubeclientConfig.QPS, kubeclientConfig.Burst))

	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(kubeclientConfig)
	if err != nil {
		return nil, err
	}

	return &KubernetesProvider{
		Client:    client,
		delimiter: providerConfig.Delimiter,
	}, nil

}

func (provider *KubernetesProvider) Start(ctx context.Context, name string) (instance.State, error) {
	config, err := provider.convertName(name)
	if err != nil {
		return instance.UnrecoverableInstanceState(name, err.Error(), int(config.Replicas))
	}

	return provider.scale(ctx, config, config.Replicas)
}

func (provider *KubernetesProvider) Stop(ctx context.Context, name string) (instance.State, error) {
	config, err := provider.convertName(name)
	if err != nil {
		return instance.UnrecoverableInstanceState(name, err.Error(), int(config.Replicas))
	}

	return provider.scale(ctx, config, 0)

}

func (provider *KubernetesProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	deployments, err := provider.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: enableLabel,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, deployment := range deployments.Items {
		groupName := deployment.Labels[groupLabel]
		if len(groupName) == 0 {
			groupName = defaultGroupValue
		}

		group := groups[groupName]
		// TOOD: Use annotation for scale
		name := provider.convertDeployment(&deployment, 1)
		group = append(group, name)
		groups[groupName] = group
	}

	statefulSets, err := provider.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: enableLabel,
	})

	if err != nil {
		return nil, err
	}

	for _, statefulSet := range statefulSets.Items {
		groupName := statefulSet.Labels[groupLabel]
		if len(groupName) == 0 {
			groupName = defaultGroupValue
		}

		group := groups[groupName]
		// TOOD: Use annotation for scale
		name := provider.convertStatefulset(&statefulSet, 1)
		group = append(group, name)
		groups[groupName] = group
	}

	return groups, nil
}

func (provider *KubernetesProvider) scale(ctx context.Context, config *Config, replicas int32) (instance.State, error) {
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

func (provider *KubernetesProvider) GetState(ctx context.Context, name string) (instance.State, error) {
	config, err := provider.convertName(name)
	if err != nil {
		return instance.UnrecoverableInstanceState(name, err.Error(), int(config.Replicas))
	}

	switch config.Kind {
	case "deployment":
		return provider.getDeploymentState(ctx, config)
	case "statefulset":
		return provider.getStatefulsetState(ctx, config)
	default:
		return instance.UnrecoverableInstanceState(config.OriginalName, fmt.Sprintf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", config.Kind), int(config.Replicas))
	}
}

func (provider *KubernetesProvider) getDeploymentState(ctx context.Context, config *Config) (instance.State, error) {
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

func (provider *KubernetesProvider) getStatefulsetState(ctx context.Context, config *Config) (instance.State, error) {
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

func (provider *KubernetesProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {

	informer := provider.watchDeployents(instance)
	go informer.Run(ctx.Done())
	informer = provider.watchStatefulSets(instance)
	go informer.Run(ctx.Done())
}

func (provider *KubernetesProvider) watchDeployents(instance chan<- string) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*appsv1.Deployment)
			oldDeployment := old.(*appsv1.Deployment)

			if newDeployment.ObjectMeta.ResourceVersion == oldDeployment.ObjectMeta.ResourceVersion {
				return
			}

			if *newDeployment.Spec.Replicas == 0 {
				instance <- provider.convertDeployment(newDeployment, *oldDeployment.Spec.Replicas)
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedDeployment := obj.(*appsv1.Deployment)
			instance <- provider.convertDeployment(deletedDeployment, *deletedDeployment.Spec.Replicas)
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(provider.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().Deployments().Informer()

	informer.AddEventHandler(handler)
	return informer
}

func (provider *KubernetesProvider) watchStatefulSets(instance chan<- string) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newStatefulSet := new.(*appsv1.StatefulSet)
			oldStatefulSet := old.(*appsv1.StatefulSet)

			if newStatefulSet.ObjectMeta.ResourceVersion == oldStatefulSet.ObjectMeta.ResourceVersion {
				return
			}

			if *newStatefulSet.Spec.Replicas == 0 {
				instance <- provider.convertStatefulset(newStatefulSet, *oldStatefulSet.Spec.Replicas)
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedStatefulSet := obj.(*appsv1.StatefulSet)
			instance <- provider.convertStatefulset(deletedStatefulSet, *deletedStatefulSet.Spec.Replicas)
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(provider.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().StatefulSets().Informer()

	informer.AddEventHandler(handler)
	return informer
}
