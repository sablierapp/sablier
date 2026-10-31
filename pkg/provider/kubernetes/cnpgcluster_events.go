package kubernetes

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// cnpgCRDState is whether the CloudNativePG Cluster CRD is served by the API server.
type cnpgCRDState int32

const (
	// cnpgCRDUnknown means discovery failed; it is not cached, so the next caller retries.
	cnpgCRDUnknown cnpgCRDState = iota
	cnpgCRDAbsent
	cnpgCRDInstalled
)

// clusterCRDState reports whether the CloudNativePG Cluster CRD is served, asking the
// discovery API only until it has an answer, which is then cached for the life of the
// provider. It is shared by InstanceEvents and listClusters, so both agree on it.
//
// The check goes through the discovery API rather than listing the resource:
// discovery requires no RBAC on the resource itself, so a ClusterRole scoped to
// deployments/statefulsets is sufficient. Probing with a List instead turns a
// missing permission into "Forbidden", which is indistinguishable from a transient
// error and used to start the informer against a CRD that does not exist - leaving
// the reflector in a permanent list/watch error loop.
//
// The cached answer does not follow the CRD being installed or removed while Sablier
// runs; that takes a restart, as starting the CloudNativePG informer already does.
func (p *Provider) clusterCRDState(ctx context.Context) (cnpgCRDState, error) {
	if state := cnpgCRDState(p.cnpgCRD.Load()); state != cnpgCRDUnknown {
		return state, nil
	}
	if p.dynamic == nil || p.Client == nil {
		return cnpgCRDAbsent, nil
	}
	resources, err := p.Client.Discovery().ServerResourcesForGroupVersionWithContext(ctx, cnpgClusterGVR.GroupVersion().String())
	if err != nil && !apierrors.IsNotFound(err) {
		return cnpgCRDUnknown, err
	}
	state := cnpgCRDAbsent
	if err == nil {
		for _, r := range resources.APIResources {
			if r.Name == cnpgClusterGVR.Resource {
				state = cnpgCRDInstalled
				break
			}
		}
	}
	p.cnpgCRD.Store(int32(state))
	return state, nil
}

// clusterCRDInstalled reports whether InstanceEvents should start the CloudNativePG
// informer. When discovery fails for another reason than NotFound (transient,
// permission, authn, ...), it assumes the CRD exists and lets the informer surface
// the underlying problem.
func (p *Provider) clusterCRDInstalled(ctx context.Context) bool {
	state, err := p.clusterCRDState(ctx)
	if state == cnpgCRDUnknown {
		p.l.WarnContext(ctx, "could not verify cloudnativepg CRD presence, enabling cnpg watcher anyway", "error", err)
		return true
	}
	return state == cnpgCRDInstalled
}

// clusterFromObject extracts the *unstructured.Unstructured from an informer event,
// unwrapping a tombstone when the final state was missed.
func clusterFromObject(obj any) (*unstructured.Unstructured, bool) {
	return eventObject[*unstructured.Unstructured](obj)
}

func (p *Provider) watchClusters(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.SharedIndexInformer {
	handler := p.clusterEventHandler(ctx, instance, wantStopped, wantStarted, wantCreated, wantRemoved)
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(p.dynamic, informerResync, metav1.NamespaceAll, nil)
	informer := factory.ForResource(cnpgClusterGVR).Informer()

	_, _ = informer.AddEventHandler(handler)
	return informer
}

func (p *Provider) clusterEventHandler(ctx context.Context, instance chan<- sablier.InstanceEvent, wantStopped, wantStarted, wantCreated, wantRemoved bool) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if !wantCreated {
				return
			}
			u, ok := clusterFromObject(obj)
			if !ok {
				return
			}
			parsed := ClusterName(u.GetNamespace(), u.GetName(), ParseOptions{Delimiter: p.delimiter})
			// Build the info from the event object, as inspect would, instead of
			// fetching it again: the informer is not filtered by label, so its
			// initial sync delivers every cnpgcluster in the cluster, and a GET for
			// each would queue behind the client-side rate limiter.
			instance <- sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: p.clusterInfo(u, parsed)}
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
					Name:      parsed.Name,
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
}
