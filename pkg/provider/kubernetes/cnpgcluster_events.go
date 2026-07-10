package kubernetes

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// clusterCRDInstalled reports whether the CloudNativePG Cluster CRD is served by the
// API server. It lets InstanceEvents avoid starting a dynamic informer (which would
// otherwise log continuous list/watch errors) on clusters that don't run CloudNativePG.
//
// The check goes through the discovery API rather than listing the resource:
// discovery requires no RBAC on the resource itself, so a ClusterRole scoped to
// deployments/statefulsets is sufficient. Probing with a List instead turns a
// missing permission into "Forbidden", which is indistinguishable from a transient
// error and used to start the informer against a CRD that does not exist - leaving
// the reflector in a permanent list/watch error loop.
//
// ctx only scopes logging: ServerResourcesForGroupVersion has no context-aware
// variant, so the underlying GET is bounded by the REST client timeout instead.
func (p *Provider) clusterCRDInstalled(ctx context.Context) bool {
	if p.dynamic == nil || p.Client == nil {
		return false
	}
	resources, err := p.Client.Discovery().ServerResourcesForGroupVersion(cnpgClusterGVR.GroupVersion().String())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
		// Any other error (transient, permission, authn, ...): assume the CRD
		// exists and let the informer surface the underlying problem.
		p.l.WarnContext(ctx, "could not verify cloudnativepg CRD presence, enabling cnpg watcher anyway", "error", err)
		return true
	}
	for _, r := range resources.APIResources {
		if r.Name == cnpgClusterGVR.Resource {
			return true
		}
	}
	return false
}

// clusterFromObject extracts the *unstructured.Unstructured from an informer event,
// unwrapping a tombstone when the final state was missed.
func clusterFromObject(obj any) (*unstructured.Unstructured, bool) {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u, true
	}
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}
	u, ok := tombstone.Obj.(*unstructured.Unstructured)
	return u, ok
}

func (p *Provider) watchClusters(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if !wantCreated {
				return
			}
			u, ok := clusterFromObject(obj)
			if !ok {
				return
			}
			parsed := ClusterName(u.GetNamespace(), u.GetName(), ParseOptions{Delimiter: p.delimiter})
			info, err := p.InstanceInspect(ctx, parsed.Original)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after add event failed, using bare info", "cnpgcluster", parsed.Original, "error", err)
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: parsed.Original, Provider: sablier.ProviderKubernetes}}
				return
			}
			instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: info}
		},
		UpdateFunc: func(old, new any) {
			newCluster, ok := clusterFromObject(new)
			if !ok {
				return
			}
			oldCluster, ok := clusterFromObject(old)
			if !ok {
				return
			}
			if newCluster.GetResourceVersion() == oldCluster.GetResourceVersion() {
				return
			}

			oldHibernating := oldCluster.GetAnnotations()[cnpgHibernationAnnotation] == cnpgHibernationOn
			newHibernating := newCluster.GetAnnotations()[cnpgHibernationAnnotation] == cnpgHibernationOn

			if wantStopped && !oldHibernating && newHibernating {
				parsed := ClusterName(newCluster.GetNamespace(), newCluster.GetName(), ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after hibernate event failed, using bare info", "cnpgcluster", parsed.Original, "error", err)
					instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderKubernetes}}
					return
				}
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}
			}
			if wantStarted && oldHibernating && !newHibernating {
				parsed := ClusterName(newCluster.GetNamespace(), newCluster.GetName(), ParseOptions{Delimiter: p.delimiter})
				info, err := p.InstanceInspect(ctx, parsed.Original)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after resume event failed, using bare info", "cnpgcluster", parsed.Original, "error", err)
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
			u, ok := clusterFromObject(obj)
			if !ok {
				return
			}
			parsed := ClusterName(u.GetNamespace(), u.GetName(), ParseOptions{Delimiter: p.delimiter})
			// Cluster is gone; build InstanceInfo from the deleted object directly.
			image, _, _ := unstructured.NestedString(u.Object, "spec", "imageName")
			labels := u.GetLabels()
			info := sablier.InstanceInfo{
				Name:     parsed.Original,
				Status:   sablier.InstanceStatusStopped,
				Provider: sablier.ProviderKubernetes,
				Kubernetes: &sablier.KubernetesWorkloadInfo{
					Namespace: u.GetNamespace(),
					Kind:      KindCNPGCluster,
					Image:     image,
					Labels:    labels,
				},
			}
			sablier.PopulateEnabledAndGroup(&info, sablierConfig(labels, u.GetAnnotations()))
			if wantRemoved {
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: info}
			}
			if wantStopped {
				instance <- sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}
			}
		},
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(p.dynamic, 2*time.Second, metav1.NamespaceAll, nil)
	informer := factory.ForResource(cnpgClusterGVR).Informer()

	_, _ = informer.AddEventHandler(handler)
	return informer
}
