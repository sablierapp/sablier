package kubernetes

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) watchDeployments(ctx context.Context, instance chan<- sablier.InstanceInfo, wantStopped, wantStarted bool) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*appsv1.Deployment)
			oldDeployment := old.(*appsv1.Deployment)

			if newDeployment.ResourceVersion == oldDeployment.ResourceVersion {
				return
			}

			oldReplicas := *oldDeployment.Spec.Replicas
			newReplicas := *newDeployment.Spec.Replicas

			if wantStopped && oldReplicas != 0 && newReplicas == 0 {
				parsed := DeploymentName(newDeployment, ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-to-0 event failed, using bare info", "deployment", parsed.Original, "error", err)
					instance <- sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderKubernetes}
					return
				}
				instance <- info
			}
			if wantStarted && oldReplicas == 0 && newReplicas != 0 {
				parsed := DeploymentName(newDeployment, ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-from-0 event failed, using bare info", "deployment", parsed.Original, "error", err)
					instance <- sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderKubernetes}
					return
				}
				instance <- info
			}
		},
		DeleteFunc: func(obj interface{}) {
			d := obj.(*appsv1.Deployment)
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
			sablier.PopulateEnabledAndGroup(&info, d.Labels)
			instance <- info
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(corev1.NamespaceAll))
	informer := factory.Apps().V1().Deployments().Informer()

	_, _ = informer.AddEventHandler(handler)
	return informer
}
