package kubernetes

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) watchStatefulSets(instance chan<- sablier.InstanceInfo) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newStatefulSet := new.(*appsv1.StatefulSet)
			oldStatefulSet := old.(*appsv1.StatefulSet)

			if newStatefulSet.ResourceVersion == oldStatefulSet.ResourceVersion {
				return
			}

			if *oldStatefulSet.Spec.Replicas == 0 {
				return
			}

			if *newStatefulSet.Spec.Replicas == 0 {
				parsed := StatefulSetName(newStatefulSet, ParseOptions{Delimiter: p.delimiter})
				instance <- sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusNotReady}
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedStatefulSet := obj.(*appsv1.StatefulSet)
			parsed := StatefulSetName(deletedStatefulSet, ParseOptions{Delimiter: p.delimiter})
			instance <- sablier.InstanceInfo{Name: parsed.Original, Status: sablier.InstanceStatusNotReady}
		},
	}
	factory := informers.NewSharedInformerFactoryWithOptions(p.Client, 2*time.Second, informers.WithNamespace(core_v1.NamespaceAll))
	informer := factory.Apps().V1().StatefulSets().Informer()

	_, _ = informer.AddEventHandler(handler)
	return informer
}
