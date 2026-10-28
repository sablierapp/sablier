package kubernetes

import (
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// eventObject extracts a typed object from an informer event, unwrapping a
// cache.DeletedFinalStateUnknown tombstone when the watch missed the final
// delete (the informer delivers tombstones on relist after a disconnect).
// An unchecked type assertion in an event handler panics inside the informer's
// processor goroutine and takes the whole process down, so every handler must
// go through this helper instead of asserting directly.
func eventObject[T any](obj any) (T, bool) {
	if t, ok := obj.(T); ok {
		return t, true
	}
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		t, ok := tombstone.Obj.(T)
		return t, ok
	}
	var zero T
	return zero, false
}

// managed reports whether a workload opted in to Sablier. It reads the enable
// key from the labels and annotations as InstanceInfo.IsEnabled does.
//
// The informers watch every workload in the cluster, so each handler checks
// the event object first and drops unmanaged workloads before it calls the API
// or reports anything. A server-side label selector cannot do this instead: it
// cannot express the OR of the enable selectors, it does not see annotations,
// and a workload that loses its label would reach DeleteFunc as if deleted.
func managed(labels, annotations map[string]string) bool {
	return sablierConfig(labels, annotations)[sablier.LabelEnable] == "true"
}

// replicasOf dereferences an optional replica count, defaulting to 1 exactly
// like the API server does when the field is unset.
func replicasOf(replicas *int32) int32 {
	if replicas == nil {
		return 1
	}
	return *replicas
}
