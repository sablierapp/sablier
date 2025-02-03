package kubernetes

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/providers"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Interface guard
var _ providers.Provider = (*Provider)(nil)

type Workload interface {
	GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

type Provider struct {
	Client    kubernetes.Interface
	delimiter string
	l         *slog.Logger
}

func NewProvider(ctx context.Context, logger *slog.Logger, providerConfig providerConfig.Kubernetes) (*Provider, error) {
	logger = logger.With(slog.String("provider", "kubernetes"))

	kubeclientConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot create in-cluster config: %w", err)
	}
	kubeclientConfig.QPS = providerConfig.QPS
	kubeclientConfig.Burst = providerConfig.Burst

	client, err := kubernetes.NewForConfig(kubeclientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubernetes client: %w", err)
	}

	info, err := client.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("cannot get server version: %w", err)
	}

	logger.InfoContext(ctx, "connection established with kubernetes",
		slog.String("version", info.String()),
		slog.Float64("config.qps", float64(kubeclientConfig.QPS)),
		slog.Int("config.burst", kubeclientConfig.Burst),
	)

	return &Provider{
		Client:    client,
		delimiter: providerConfig.Delimiter,
		l:         logger,
	}, nil
}

func (p *Provider) Start(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return fmt.Errorf("cannot parse name: %w", err)
	}

	return p.scale(ctx, parsed, parsed.Replicas)
}

func (p *Provider) Stop(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return fmt.Errorf("cannot parse name: %w", err)
	}

	return p.scale(ctx, parsed, 0)
}

func (p *Provider) GetGroups(ctx context.Context) (map[string][]string, error) {
	deployments, err := p.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: discovery.LabelEnable,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list deployments: %w", err)
	}

	groups := make(map[string][]string)
	for _, deployment := range deployments.Items {
		groupName := deployment.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := DeploymentName(deployment, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: discovery.LabelEnable,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list statefulsets: %w", err)
	}

	for _, statefulSet := range statefulSets.Items {
		groupName := statefulSet.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := StatefulSetName(statefulSet, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	return groups, nil
}

func (p *Provider) scale(ctx context.Context, config ParsedName, replicas int32) error {
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
		return fmt.Errorf("cannot get scale: %w", err)
	}

	s.Spec.Replicas = replicas
	_, err = workload.UpdateScale(ctx, config.Name, s, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cannot update scale: %w", err)
	}

	return nil
}

func (p *Provider) GetState(ctx context.Context, name string) (instance.State, error) {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return instance.State{}, fmt.Errorf("cannot parse name: %w", err)
	}

	switch parsed.Kind {
	case "deployment":
		return p.getDeploymentState(ctx, parsed)
	case "statefulset":
		return p.getStatefulsetState(ctx, parsed)
	default:
		return instance.State{}, fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", parsed.Kind)
	}
}

func (p *Provider) getDeploymentState(ctx context.Context, config ParsedName) (instance.State, error) {
	d, err := p.Client.AppsV1().Deployments(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return instance.State{}, fmt.Errorf("cannot get deployment: %w", err)
	}

	if *d.Spec.Replicas == d.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.Original, config.Replicas), nil
	}

	return instance.NotReadyInstanceState(config.Original, d.Status.ReadyReplicas, config.Replicas), nil
}

func (p *Provider) getStatefulsetState(ctx context.Context, config ParsedName) (instance.State, error) {
	ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return instance.State{}, fmt.Errorf("cannot get statefulset: %w", err)
	}

	if *ss.Spec.Replicas == ss.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.Original, ss.Status.ReadyReplicas), nil
	}

	return instance.NotReadyInstanceState(config.Original, ss.Status.ReadyReplicas, *ss.Spec.Replicas), nil
}

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	informer := p.watchDeployments(instance)
	go informer.Run(ctx.Done())
	informer = p.watchStatefulSets(instance)
	go informer.Run(ctx.Done())
}

func (p *Provider) watchDeployments(instance chan<- string) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*appsv1.Deployment)
			oldDeployment := old.(*appsv1.Deployment)

			if newDeployment.ObjectMeta.ResourceVersion == oldDeployment.ObjectMeta.ResourceVersion {
				return
			}

			if *newDeployment.Spec.Replicas == 0 {
				parsed := DeploymentName(*newDeployment, ParseOptions{Delimiter: p.delimiter})
				instance <- parsed.Original
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedDeployment := obj.(*appsv1.Deployment)
			parsed := DeploymentName(*deletedDeployment, ParseOptions{Delimiter: p.delimiter})
			instance <- parsed.Original
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().Deployments().Informer()

	informer.AddEventHandler(handler)
	return informer
}

func (p *Provider) watchStatefulSets(instance chan<- string) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newStatefulSet := new.(*appsv1.StatefulSet)
			oldStatefulSet := old.(*appsv1.StatefulSet)

			if newStatefulSet.ObjectMeta.ResourceVersion == oldStatefulSet.ObjectMeta.ResourceVersion {
				return
			}

			if *newStatefulSet.Spec.Replicas == 0 {
				parsed := StatefulSetName(*newStatefulSet, ParseOptions{Delimiter: p.delimiter})
				instance <- parsed.Original
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedStatefulSet := obj.(*appsv1.StatefulSet)
			parsed := StatefulSetName(*deletedStatefulSet, ParseOptions{Delimiter: p.delimiter})
			instance <- parsed.Original
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().StatefulSets().Informer()

	informer.AddEventHandler(handler)
	return informer
}
