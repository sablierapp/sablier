package kubernetes

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/pkg/provider"
	"log/slog"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"

	"github.com/sablierapp/sablier/app/instance"
	providerConfig "github.com/sablierapp/sablier/config"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Interface guard
var _ provider.Provider = (*KubernetesProvider)(nil)

type Workload interface {
	GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

type KubernetesProvider struct {
	Client    kubernetes.Interface
	delimiter string
	l         *slog.Logger
}

func NewKubernetesProvider(ctx context.Context, client *kubernetes.Clientset, logger *slog.Logger, kubeclientConfig providerConfig.Kubernetes) (*KubernetesProvider, error) {
	logger = logger.With(slog.String("provider", "kubernetes"))

	info, err := client.ServerVersion()
	if err != nil {
		return nil, err
	}

	logger.InfoContext(ctx, "connection established with kubernetes",
		slog.String("version", info.String()),
		slog.Float64("config.qps", float64(kubeclientConfig.QPS)),
		slog.Int("config.burst", kubeclientConfig.Burst),
	)

	return &KubernetesProvider{
		Client:    client,
		delimiter: kubeclientConfig.Delimiter,
		l:         logger,
	}, nil

}

func (p *KubernetesProvider) Start(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	return p.scale(ctx, parsed, parsed.Replicas)
}

func (p *KubernetesProvider) Stop(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	return p.scale(ctx, parsed, 0)

}

func (p *KubernetesProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	deployments, err := p.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: discovery.LabelEnable,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, deployment := range deployments.Items {
		groupName := deployment.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := DeploymentName(&deployment, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: discovery.LabelEnable,
	})

	if err != nil {
		return nil, err
	}

	for _, statefulSet := range statefulSets.Items {
		groupName := statefulSet.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := StatefulSetName(&statefulSet, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	return groups, nil
}

func (p *KubernetesProvider) scale(ctx context.Context, config ParsedName, replicas int32) error {
	var workload Workload

	switch config.Kind {
	case "deployment":
		workload = p.Client.AppsV1().Deployments(config.Namespace)
	case "statefulset":
		workload = p.Client.AppsV1().StatefulSets(config.Namespace)
	default:
		return fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", config.Kind)
	}

	s, err := workload.GetScale(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	s.Spec.Replicas = replicas
	_, err = workload.UpdateScale(ctx, config.Name, s, metav1.UpdateOptions{})

	return err
}

func (p *KubernetesProvider) GetState(ctx context.Context, name string) (instance.State, error) {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return instance.State{}, err
	}

	switch parsed.Kind {
	case "deployment":
		return p.DeploymentInspect(ctx, parsed)
	case "statefulset":
		return p.getStatefulsetState(ctx, parsed)
	default:
		return instance.State{}, fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", parsed.Kind)
	}
}

func (p *KubernetesProvider) getStatefulsetState(ctx context.Context, config ParsedName) (instance.State, error) {
	ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return instance.State{}, err
	}

	if *ss.Spec.Replicas == ss.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.Original, ss.Status.ReadyReplicas), nil
	}

	return instance.NotReadyInstanceState(config.Original, ss.Status.ReadyReplicas, *ss.Spec.Replicas), nil
}

func (p *KubernetesProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {

	informer := p.watchDeployents(instance)
	go informer.Run(ctx.Done())
	informer = p.watchStatefulSets(instance)
	go informer.Run(ctx.Done())
}

func (p *KubernetesProvider) watchDeployents(instance chan<- string) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*appsv1.Deployment)
			oldDeployment := old.(*appsv1.Deployment)

			if newDeployment.ObjectMeta.ResourceVersion == oldDeployment.ObjectMeta.ResourceVersion {
				return
			}

			if *newDeployment.Spec.Replicas == 0 {
				parsed := DeploymentName(newDeployment, ParseOptions{Delimiter: p.delimiter})
				instance <- parsed.Original
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedDeployment := obj.(*appsv1.Deployment)
			parsed := DeploymentName(deletedDeployment, ParseOptions{Delimiter: p.delimiter})
			instance <- parsed.Original
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().Deployments().Informer()

	informer.AddEventHandler(handler)
	return informer
}

func (p *KubernetesProvider) watchStatefulSets(instance chan<- string) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newStatefulSet := new.(*appsv1.StatefulSet)
			oldStatefulSet := old.(*appsv1.StatefulSet)

			if newStatefulSet.ObjectMeta.ResourceVersion == oldStatefulSet.ObjectMeta.ResourceVersion {
				return
			}

			if *newStatefulSet.Spec.Replicas == 0 {
				parsed := StatefulSetName(newStatefulSet, ParseOptions{Delimiter: p.delimiter})
				instance <- parsed.Original
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedStatefulSet := obj.(*appsv1.StatefulSet)
			parsed := StatefulSetName(deletedStatefulSet, ParseOptions{Delimiter: p.delimiter})
			instance <- parsed.Original
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().StatefulSets().Informer()

	informer.AddEventHandler(handler)
	return informer
}
