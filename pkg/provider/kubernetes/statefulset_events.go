package kubernetes

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) watchStatefulSets(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !wantCreated {
				return
			}
			ss := obj.(*appsv1.StatefulSet)
			parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})
			info, err := p.InstanceInspect(ctx, parsed.Original)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after add event failed, using bare info", "statefulset", parsed.Original, "error", err)
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: parsed.Original, Provider: sablier.ProviderKubernetes}}
				return
			}
			instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: info}
		},
		UpdateFunc: func(old, new interface{}) {
			newStatefulSet := new.(*appsv1.StatefulSet)
			oldStatefulSet := old.(*appsv1.StatefulSet)

			if newStatefulSet.ResourceVersion == oldStatefulSet.ResourceVersion {
				return
			}

			oldReplicas := *oldStatefulSet.Spec.Replicas
			newReplicas := *newStatefulSet.Spec.Replicas

			if wantStopped && oldReplicas != 0 && newReplicas == 0 {
				parsed := StatefulSetName(newStatefulSet, ParseOptions{Delimiter: p.delimiter})
				// StatefulSet still exists (scaled to 0); inspect for full info.
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-to-0 event failed, using bare info", "statefulset", parsed.Original, "error", err)
					instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderKubernetes}}
					return
				}
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}
			}
			if wantStarted && oldReplicas == 0 && newReplicas != 0 {
				parsed := StatefulSetName(newStatefulSet, ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-from-0 event failed, using bare info", "statefulset", parsed.Original, "error", err)
					instance <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderKubernetes}}
					return
				}
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: info}
			}
		},
		DeleteFunc: func(obj interface{}) {
			if !wantRemoved {
				return
			}
			ss := obj.(*appsv1.StatefulSet)
			parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})
			// StatefulSet is gone; build InstanceInfo from the deleted object directly.
			var image string
			if len(ss.Spec.Template.Spec.Containers) > 0 {
				image = ss.Spec.Template.Spec.Containers[0].Image
			}
			info := sablier.InstanceInfo{
				Name:     parsed.Original,
				Status:   sablier.InstanceStatusStopped,
				Provider: sablier.ProviderKubernetes,
				Kubernetes: &sablier.KubernetesWorkloadInfo{
					Namespace: ss.Namespace,
					Kind:      "statefulset",
					Image:     image,
					Labels:    ss.Labels,
				},
			}
			sablier.PopulateEnabledAndGroup(&info, ss.Labels)
			instance <- sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: info}
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().StatefulSets().Informer()

	_, _ = informer.AddEventHandler(handler)
	return informer
}
