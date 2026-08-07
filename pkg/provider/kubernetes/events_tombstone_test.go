package kubernetes

import (
	"log/slog"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// The informer machinery delivers cache.DeletedFinalStateUnknown tombstones to
// DeleteFunc when a watch disconnect made it miss the final delete (relist).
// These tests feed a tombstone through the real handlers: before the fix the
// unchecked type assertions panicked inside the informer's processor
// goroutine, killing the whole process.

func tombstoneTestProvider() *Provider {
	return &Provider{l: slog.New(slog.DiscardHandler), delimiter: "_"}
}

func TestDeploymentDeleteHandlesTombstone(t *testing.T) {
	t.Parallel()

	p := tombstoneTestProvider()
	events := make(chan sablier.InstanceEvent, 4)
	handler := p.deploymentEventHandler(t.Context(), events, true, true, true, true)

	deleted := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "demo"},
	}
	handler.OnDelete(cache.DeletedFinalStateUnknown{Key: "demo/myapp", Obj: deleted})

	ev := <-events
	assert.Equal(t, provider.InstanceEventRemoved, ev.Type)
	assert.Equal(t, "deployment_demo_myapp_1", ev.Info.Name)
	ev = <-events
	assert.Equal(t, provider.InstanceEventStopped, ev.Type)
}

func TestStatefulSetDeleteHandlesTombstone(t *testing.T) {
	t.Parallel()

	p := tombstoneTestProvider()
	events := make(chan sablier.InstanceEvent, 4)
	handler := p.statefulSetEventHandler(t.Context(), events, true, true, true, true)

	deleted := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "mydb", Namespace: "demo"},
	}
	handler.OnDelete(cache.DeletedFinalStateUnknown{Key: "demo/mydb", Obj: deleted})

	ev := <-events
	assert.Equal(t, provider.InstanceEventRemoved, ev.Type)
	assert.Equal(t, "statefulset_demo_mydb_1", ev.Info.Name)
	ev = <-events
	assert.Equal(t, provider.InstanceEventStopped, ev.Type)
}

// TestUpdateHandlesNilReplicas pins that an update event with a nil
// Spec.Replicas (legal: the field is a *int32 defaulted server-side) does not
// panic the handler.
func TestUpdateHandlesNilReplicas(t *testing.T) {
	t.Parallel()

	p := tombstoneTestProvider()
	events := make(chan sablier.InstanceEvent, 4)
	handler := p.deploymentEventHandler(t.Context(), events, false, false, false, false)

	oldD := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "demo", ResourceVersion: "1"}}
	newD := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "demo", ResourceVersion: "2"}}
	handler.OnUpdate(oldD, newD) // Spec.Replicas nil on both: must not panic
}
