package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) watchDeployments(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.SharedIndexInformer {
	handler := p.deploymentEventHandler(ctx, instance, wantStopped, wantStarted, wantCreated, wantRemoved)
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, informerResync, informers.WithNamespace(corev1.NamespaceAll))
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
			if !ok || !managed(d.Labels, d.Annotations) {
				return
			}
			parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})
			// Build the info from the event object, as inspect would, instead of
			// fetching it again: the informer's initial sync delivers every
			// managed deployment, and a GET for each would queue behind the
			// client-side rate limiter.
			instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: p.deploymentInfo(d, parsed)}
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
			// Managed on either side is enough: an update that removes the
			// enable label while scaling to 0 still stops a managed instance,
			// and one that adds it while scaling from 0 starts one.
			if !managed(oldDeployment.Labels, oldDeployment.Annotations) && !managed(newDeployment.Labels, newDeployment.Annotations) {
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
			// The deleted object, or the tombstone's last known state, still
			// carries the labels and annotations that made it managed.
			d, ok := eventObject[*appsv1.Deployment](obj)
			if !ok || !managed(d.Labels, d.Annotations) {
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
					Name:      parsed.Name,
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
