package kubernetes

import (
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// The informers are not filtered by label, so their initial sync delivers an
// Add event for every workload in the cluster. The Created event must be built
// from the event object, without a GET, and match what InstanceInspect reports.
func TestAddEvent_BuildsInfoFromObjectWithoutGet(t *testing.T) {
	t.Parallel()

	two := int32(2)
	labels := map[string]string{"sablier.enable": "true", "sablier.group": "media"}
	annotations := map[string]string{"sablier.anti-affinity": "streaming"}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nextcloud", Labels: labels, Annotations: annotations},
		Spec:       appsv1.DeploymentSpec{Replicas: &two},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	}
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "immich", Labels: labels, Annotations: annotations},
		Spec:       appsv1.StatefulSetSpec{Replicas: &two},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	cluster := newClusterObj("default", "pg", labels, annotations, 1, 1)

	p := newFakeCNPGProvider(t, cluster)
	client := k8sfake.NewSimpleClientset(d, ss)
	p.Client = client
	dyn := p.dynamic.(*dynamicfake.FakeDynamicClient)

	for _, tc := range []struct {
		name    string
		handler func(chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs
		obj     any
	}{
		{"deployment", func(ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs {
			return p.deploymentEventHandler(t.Context(), ch, false, false, true, false)
		}, d},
		{"statefulset", func(ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs {
			return p.statefulSetEventHandler(t.Context(), ch, false, false, true, false)
		}, ss},
		{"cnpgcluster", func(ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs {
			return p.clusterEventHandler(t.Context(), ch, false, false, true, false)
		}, cluster},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := make(chan sablier.InstanceEvent, 1)
			handler := tc.handler(events)
			client.ClearActions()
			dyn.ClearActions()

			handler.AddFunc(tc.obj)
			ev := <-events

			assert.Equal(t, len(client.Actions())+len(dyn.Actions()), 0, "the Add event must not call the API")
			assert.Equal(t, ev.Type, provider.InstanceEventCreated)
			want, err := p.InstanceInspect(t.Context(), ev.Info.Name)
			assert.NilError(t, err)
			assert.DeepEqual(t, ev.Info, want)
			assert.DeepEqual(t, ev.Info.AntiAffinity, []string{"streaming"})
		})
	}
}
