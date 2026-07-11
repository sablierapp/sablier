package kubernetes

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) watchDeployments(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.SharedIndexInformer {
	handler := p.deploymentEventHandler(ctx, instance, wantStopped, wantStarted, wantCreated, wantRemoved)
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(corev1.NamespaceAll))
	informer := factory.Apps().V1().Deployments().Informer()

	_, _ = informer.AddEventHandler(handler)
	return informer
}

func (p *Provider) deploymentEventHandler(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if !wantCreated {
				return
			}
			d, ok := eventObject[*appsv1.Deployment](obj)
			if !ok {
				return
			}
			parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})
			info, err := p.InstanceInspect(ctx, parsed.Original)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after add event failed, using bare info", "deployment", parsed.Original, "error", err)
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: parsed.Original, Provider: sablier.ProviderKubernetes}}
				return
			}
			instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: info}
		},
		UpdateFunc: func(old, new any) {
			newDeployment, ok := eventObject[*appsv1.Deployment](new)
			if !ok {
				return
			}
			oldDeployment, ok := eventObject[*appsv1.Deployment](old)
			if !ok {
				return
			}

			if newDeployment.ResourceVersion == oldDeployment.ResourceVersion {
				return
			}

			oldReplicas := replicasOf(oldDeployment.Spec.Replicas)
			newReplicas := replicasOf(newDeployment.Spec.Replicas)

			if wantStopped && oldReplicas != 0 && newReplicas == 0 {
				parsed := DeploymentName(newDeployment, ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-to-0 event failed, using bare info", "deployment", parsed.Original, "error", err)
					instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderKubernetes}}
					return
				}
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}
			}
			if wantStarted && oldReplicas == 0 && newReplicas != 0 {
				parsed := DeploymentName(newDeployment, ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-from-0 event failed, using bare info", "deployment", parsed.Original, "error", err)
					instance <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderKubernetes}}
					return
				}
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: info}
			}
		},
		DeleteFunc: func(obj any) {
			if !wantRemoved && !wantStopped {
				return
			}
			d, ok := eventObject[*appsv1.Deployment](obj)
			if !ok {
				return
			}
			parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})
			// Deployment is gone; build InstanceInfo from the deleted object directly.
			var image string
			if len(d.Spec.Template.Spec.Containers) > 0 {
				image = d.Spec.Template.Spec.Containers[0].Image
			}
			info := sablier.InstanceInfo{
				Name:     parsed.Original,
				Status:   sablier.InstanceStatusStopped,
				Provider: sablier.ProviderKubernetes,
				Kubernetes: &sablier.KubernetesWorkloadInfo{
					Namespace: d.Namespace,
					Kind:      "deployment",
					Image:     image,
					Labels:    d.Labels,
				},
			}
			sablier.PopulateEnabledAndGroup(&info, sablierConfig(d.Labels, d.Annotations))
			if wantRemoved {
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: info}
			}
			if wantStopped {
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}
			}
		},
	}
}
