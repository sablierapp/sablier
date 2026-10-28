package kubernetes

import (
	"context"
	"maps"
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// The informers watch every workload in the cluster. These tests pin that only
// the workloads that opted in to Sablier, by label or annotation, reach the
// instance event stream, and that the others cost no API call.

// buildWorkload returns a workload of one kind. A stopped workload is scaled
// to 0, or hibernated for a cnpgcluster.
type buildWorkload func(labels, annotations map[string]string, running bool, resourceVersion string) runtime.Object

type eventHandlerFor func(ctx context.Context, p *Provider, ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs

var eventTestKinds = []struct {
	name    string
	build   buildWorkload
	handler eventHandlerFor
}{
	{"deployment", testDeployment, func(ctx context.Context, p *Provider, ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs {
		return p.deploymentEventHandler(ctx, ch, true, true, true, true)
	}},
	{"statefulset", testStatefulSet, func(ctx context.Context, p *Provider, ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs {
		return p.statefulSetEventHandler(ctx, ch, true, true, true, true)
	}},
	{"cnpgcluster", testCluster, func(ctx context.Context, p *Provider, ch chan<- sablier.InstanceEvent) cache.ResourceEventHandlerFuncs {
		return p.clusterEventHandler(ctx, ch, true, true, true, true)
	}},
}

func testDeployment(labels, annotations map[string]string, running bool, resourceVersion string) runtime.Object {
	replicas := int32(0)
	if running {
		replicas = 1
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "app", ResourceVersion: resourceVersion, Labels: labels, Annotations: annotations},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}
}

func testStatefulSet(labels, annotations map[string]string, running bool, resourceVersion string) runtime.Object {
	replicas := int32(0)
	if running {
		replicas = 1
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db", ResourceVersion: resourceVersion, Labels: labels, Annotations: annotations},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
	}
}

func testCluster(labels, annotations map[string]string, running bool, resourceVersion string) runtime.Object {
	annotations = maps.Clone(annotations)
	if !running {
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[cnpgHibernationAnnotation] = cnpgHibernationOn
	}
	u := newClusterObj("default", "pg", labels, annotations, 1, 1)
	u.SetResourceVersion(resourceVersion)
	return u
}

// newEventTestProvider returns a provider backed by fake clients that hold
// stored, and a function that counts the API calls made since.
func newEventTestProvider(t *testing.T, stored runtime.Object) (*Provider, func() int) {
	t.Helper()
	var typed, dynamic []runtime.Object
	switch stored.(type) {
	case nil:
	case *unstructured.Unstructured:
		dynamic = append(dynamic, stored)
	default:
		typed = append(typed, stored)
	}
	p := newFakeCNPGProvider(t, dynamic...)
	client := k8sfake.NewSimpleClientset(typed...)
	p.Client = client
	dyn := p.dynamic.(*dynamicfake.FakeDynamicClient)
	client.ClearActions()
	dyn.ClearActions()
	return p, func() int { return len(client.Actions()) + len(dyn.Actions()) }
}

// replayLifecycle feeds a workload's lifecycle through the handler: added,
// stopped, started, deleted, then deleted again through a tombstone, as after
// a watch disconnect. It returns the reported events.
func replayLifecycle(handler cache.ResourceEventHandlerFuncs, events chan sablier.InstanceEvent, build buildWorkload, labels, annotations map[string]string) []sablier.InstanceEvent {
	handler.OnAdd(build(labels, annotations, true, "1"), true)
	handler.OnUpdate(build(labels, annotations, true, "1"), build(labels, annotations, false, "2"))
	handler.OnUpdate(build(labels, annotations, false, "2"), build(labels, annotations, true, "3"))
	handler.OnDelete(build(labels, annotations, true, "3"))
	handler.OnDelete(cache.DeletedFinalStateUnknown{Key: "default/workload", Obj: build(labels, annotations, true, "3")})
	return drain(events)
}

// drain returns the events already sent: the handlers send synchronously to a
// buffered channel.
func drain(events chan sablier.InstanceEvent) []sablier.InstanceEvent {
	close(events)
	var got []sablier.InstanceEvent
	for ev := range events {
		got = append(got, ev)
	}
	return got
}

func eventTypes(events []sablier.InstanceEvent) []provider.InstanceEventType {
	types := make([]provider.InstanceEventType, 0, len(events))
	for _, ev := range events {
		types = append(types, ev.Type)
	}
	return types
}

func TestEvents_UnmanagedWorkloadsAreIgnored(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
	}{
		{name: "no sablier keys", labels: map[string]string{"app": "web"}},
		{name: "enable label is false", labels: map[string]string{"sablier.enable": "false"}},
		// Annotations take precedence over labels (see sablierConfig).
		{name: "enable annotation overrides the label", labels: map[string]string{"sablier.enable": "true"}, annotations: map[string]string{"sablier.enable": "false"}},
		{name: "group label without enable", labels: map[string]string{"sablier.group": "media"}},
	}
	for _, kind := range eventTestKinds {
		for _, tc := range cases {
			t.Run(kind.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				p, apiCalls := newEventTestProvider(t, nil)
				events := make(chan sablier.InstanceEvent, 16)

				got := replayLifecycle(kind.handler(t.Context(), p, events), events, kind.build, tc.labels, tc.annotations)

				assert.DeepEqual(t, eventTypes(got), []provider.InstanceEventType{})
				assert.Equal(t, apiCalls(), 0, "an unmanaged workload must not call the API")
			})
		}
	}
}

func TestEvents_ManagedWorkloadsAreReported(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
	}{
		{name: "legacy label", labels: map[string]string{"sablier.enable": "true"}},
		{name: "public label", labels: map[string]string{"sablierapp.dev/enable": "true"}},
		{name: "legacy annotation", annotations: map[string]string{"sablier.enable": "true"}},
		{name: "public annotation", annotations: map[string]string{"sablierapp.dev/enable": "true"}},
	}
	want := []provider.InstanceEventType{
		provider.InstanceEventCreated,
		provider.InstanceEventStopped,
		provider.InstanceEventStarted,
		provider.InstanceEventRemoved,
		provider.InstanceEventStopped,
		provider.InstanceEventRemoved,
		provider.InstanceEventStopped,
	}
	for _, kind := range eventTestKinds {
		for _, tc := range cases {
			t.Run(kind.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				p, _ := newEventTestProvider(t, kind.build(tc.labels, tc.annotations, true, ""))
				events := make(chan sablier.InstanceEvent, 16)

				got := replayLifecycle(kind.handler(t.Context(), p, events), events, kind.build, tc.labels, tc.annotations)

				assert.DeepEqual(t, eventTypes(got), want)
				for _, ev := range got {
					assert.Assert(t, ev.Info.IsEnabled(), "%s event must carry an enabled instance", ev.Type)
				}
			})
		}
	}
}

// An update decides from both sides: a workload managed before or after the
// update is reported, so its sessions and webhooks see the transition even
// when the enable label changes in the same update as the replica count.
func TestEvents_UpdateChangingEnablement(t *testing.T) {
	t.Parallel()

	enabled := map[string]string{"sablier.enable": "true"}
	for _, kind := range eventTestKinds {
		t.Run(kind.name+"/disabled while stopping", func(t *testing.T) {
			t.Parallel()
			p, _ := newEventTestProvider(t, kind.build(nil, nil, false, ""))
			events := make(chan sablier.InstanceEvent, 16)
			handler := kind.handler(t.Context(), p, events)

			handler.OnUpdate(kind.build(enabled, nil, true, "1"), kind.build(nil, nil, false, "2"))

			assert.DeepEqual(t, eventTypes(drain(events)), []provider.InstanceEventType{provider.InstanceEventStopped})
		})
		t.Run(kind.name+"/enabled while starting", func(t *testing.T) {
			t.Parallel()
			p, _ := newEventTestProvider(t, kind.build(enabled, nil, true, ""))
			events := make(chan sablier.InstanceEvent, 16)
			handler := kind.handler(t.Context(), p, events)

			handler.OnUpdate(kind.build(nil, nil, false, "1"), kind.build(enabled, nil, true, "2"))

			got := drain(events)
			assert.DeepEqual(t, eventTypes(got), []provider.InstanceEventType{provider.InstanceEventStarted})
			assert.Assert(t, got[0].Info.IsEnabled())
		})
	}
}
