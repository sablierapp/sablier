package kubernetes

// Unit tests for --provider.kubernetes.readiness=endpoints. These run without a
// real Kubernetes cluster.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var webLabels = map[string]string{"app": "web"}

func webDeployment(replicas, ready int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns", UID: "web-uid"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: webLabels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: webLabels}},
		},
		Status: appsv1.DeploymentStatus{Replicas: replicas, ReadyReplicas: ready},
	}
}

// replicaSetPod is a ready pod of the ReplicaSet <deployment>-<hash>, as the
// Deployment controller names it.
func replicaSetPod(name, uid, deployment, hash string, labels map[string]string) *corev1.Pod {
	l := map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: hash}
	for k, v := range labels {
		l[k] = v
	}
	return readyPod(name, uid, l, metav1.OwnerReference{
		APIVersion: "apps/v1", Kind: "ReplicaSet", Name: deployment + "-" + hash, UID: types.UID(deployment + "-rs"), Controller: ptr(true),
	})
}

func webPod(name, uid string) *corev1.Pod {
	return replicaSetPod(name, uid, "web", "h1", webLabels)
}

func readyPod(name, uid string, labels map[string]string, owner metav1.OwnerReference) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: types.UID(uid), Labels: labels, OwnerReferences: []metav1.OwnerReference{owner}},
		Status:     corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
}

func notReady(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
	return pod
}

func service(name string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec:       corev1.ServiceSpec{Selector: selector},
	}
}

// endpoint targets the pod name/uid. ready nil leaves the condition unset.
func endpoint(podName, uid string, ready *bool) discoveryv1.Endpoint {
	return discoveryv1.Endpoint{
		Addresses:  []string{"10.0.0.1"},
		Conditions: discoveryv1.EndpointConditions{Ready: ready},
		TargetRef:  &corev1.ObjectReference{Kind: "Pod", Namespace: "ns", Name: podName, UID: types.UID(uid)},
	}
}

func endpointSlice(name, svc string, endpoints ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta:  metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: map[string]string{discoveryv1.LabelServiceName: svc}},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   endpoints,
	}
}

func ptr[T any](v T) *T { return &v }

func endpointsProvider(objects ...runtime.Object) *Provider {
	return &Provider{
		Client:           fake.NewSimpleClientset(objects...),
		delimiter:        "_",
		readyOnEndpoints: true,
		l:                slog.New(slog.DiscardHandler),
	}
}

func deploymentName(d *appsv1.Deployment) string {
	return DeploymentName(d, ParseOptions{Delimiter: "_"}).Original
}

func TestInstanceInspectReadiness_Deployment(t *testing.T) {
	tests := []struct {
		name                string
		readyOnFirstReplica bool
		objects             []runtime.Object
		want                sablier.InstanceStatus
		wantCurrent         int32
		wantMessage         string
	}{
		{
			name:        "no service selects the workload",
			objects:     []runtime.Object{webDeployment(1, 1), webPod("web-1", "u1")},
			want:        sablier.InstanceStatusReady,
			wantCurrent: 1,
		},
		{
			name: "service without endpoint slice",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels),
			},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "web" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "endpoint not ready yet",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(false))),
			},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "web" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "endpoint ready",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true))),
			},
			want:        sablier.InstanceStatusReady,
			wantCurrent: 1,
		},
		{
			name: "unset ready condition means ready",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", nil)),
			},
			want:        sablier.InstanceStatusReady,
			wantCurrent: 1,
		},
		{
			// The canary pods match the selector of web too: only the
			// controller owner reference tells them apart.
			name: "ready endpoint of a canary with an overlapping selector",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"),
				replicaSetPod("web-canary-1", "c1", "web-canary", "h2", map[string]string{"app": "web", "track": "canary"}),
				service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(false)), endpoint("web-canary-1", "c1", ptr(true))),
			},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "web" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "stale endpoint of a former pod with the same name",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u2"), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true))),
			},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "web" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "one of two replicas routed",
			objects: []runtime.Object{
				webDeployment(2, 2), webPod("web-1", "u1"), webPod("web-2", "u2"), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true)), endpoint("web-2", "u2", ptr(false))),
			},
			want:        sablier.InstanceStatusStarting,
			wantCurrent: 1,
			wantMessage: `waiting for service "web" to route to the workload: 1 of 2 pods routed`,
		},
		{
			name:                "one of two replicas routed with ready-on-first-replica",
			readyOnFirstReplica: true,
			objects: []runtime.Object{
				webDeployment(2, 1), webPod("web-1", "u1"), notReady(webPod("web-2", "u2")), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true)), endpoint("web-2", "u2", ptr(false))),
			},
			want:        sablier.InstanceStatusReady,
			wantCurrent: 1,
		},
		{
			name: "a pod listed in two slices counts once",
			objects: []runtime.Object{
				webDeployment(2, 2), webPod("web-1", "u1"), webPod("web-2", "u2"), service("web", webLabels),
				endpointSlice("web-v4", "web", endpoint("web-1", "u1", ptr(true))),
				endpointSlice("web-v6", "web", endpoint("web-1", "u1", ptr(true))),
			},
			want:        sablier.InstanceStatusStarting,
			wantCurrent: 1,
			wantMessage: `waiting for service "web" to route to the workload: 1 of 2 pods routed`,
		},
		{
			name: "every selecting service must route",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"),
				service("web", webLabels), service("web-headless", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true))),
				endpointSlice("web-headless-a", "web-headless", endpoint("web-1", "u1", ptr(false))),
			},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "web-headless" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "services that do not select the workload are ignored",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"),
				service("api", map[string]string{"app": "api"}),
				service("external", nil),
			},
			want:        sablier.InstanceStatusReady,
			wantCurrent: 1,
		},
		{
			name: "an ExternalName service never gets endpoints and is ignored",
			objects: []runtime.Object{
				webDeployment(1, 1), webPod("web-1", "u1"),
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "web-alias", Namespace: "ns"},
					Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "web.example.com", Selector: webLabels},
				},
			},
			want:        sablier.InstanceStatusReady,
			wantCurrent: 1,
		},
		{
			// A Service with publishNotReadyAddresses lists a pod as ready
			// before it passes its readiness probe.
			name: "a ready endpoint of a pod that is not ready",
			objects: []runtime.Object{
				webDeployment(1, 1), notReady(webPod("web-1", "u1")), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true))),
			},
			want:        sablier.InstanceStatusStarting,
			wantMessage: "waiting for the pods of the workload: 0 of 1 ready",
		},
		{
			name: "replicas not ready yet",
			objects: []runtime.Object{
				webDeployment(1, 0), webPod("web-1", "u1"), service("web", webLabels),
				endpointSlice("web-a", "web", endpoint("web-1", "u1", ptr(true))),
			},
			want: sablier.InstanceStatusStarting,
		},
		{
			name: "scaled to zero",
			objects: []runtime.Object{
				webDeployment(0, 0), service("web", webLabels),
			},
			want: sablier.InstanceStatusStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := endpointsProvider(tt.objects...)
			p.readyOnFirstReplica = tt.readyOnFirstReplica
			d := tt.objects[0].(*appsv1.Deployment)

			info, err := p.InstanceInspectReadiness(context.Background(), deploymentName(d))
			assert.NilError(t, err)
			assert.Equal(t, info.Status, tt.want)
			assert.Equal(t, info.CurrentReplicas, tt.wantCurrent)
			assert.Equal(t, info.Message, tt.wantMessage)
		})
	}
}

// listed reports whether client listed resource.
func listed(client *fake.Clientset, resource string) bool {
	for _, action := range client.Actions() {
		if action.GetVerb() == "list" && action.GetResource().Resource == resource {
			return true
		}
	}
	return false
}

// The endpoints are only checked once the replicas are ready, so a workload
// that is still starting costs no extra API request.
func TestInstanceInspectReadiness_SkipsLookupsUntilReplicasReady(t *testing.T) {
	p := endpointsProvider(webDeployment(1, 0), service("web", webLabels))

	_, err := p.InstanceInspectReadiness(context.Background(), deploymentName(webDeployment(1, 0)))
	assert.NilError(t, err)

	for _, action := range p.Client.(*fake.Clientset).Actions() {
		assert.Equal(t, action.GetResource().Resource, "deployments", "unexpected %s %s", action.GetVerb(), action.GetResource().Resource)
	}
}

// Only the request path checks the endpoints: the other callers of
// InstanceInspect, such as the running-hours or the expiration checks, keep
// one request per workload.
func TestDeploymentInspect_DoesNotCheckEndpoints(t *testing.T) {
	p := endpointsProvider(webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels))

	info, err := p.InstanceInspect(context.Background(), deploymentName(webDeployment(1, 1)))
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusReady)

	for _, action := range p.Client.(*fake.Clientset).Actions() {
		assert.Equal(t, action.GetResource().Resource, "deployments", "unexpected %s %s", action.GetVerb(), action.GetResource().Resource)
	}
}

// Without --provider.kubernetes.readiness=endpoints, the replicas alone decide.
func TestInstanceInspectReadiness_ReadinessReplicas_IgnoresEndpoints(t *testing.T) {
	p := endpointsProvider(webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels))
	p.readyOnEndpoints = false

	info, err := p.InstanceInspectReadiness(context.Background(), deploymentName(webDeployment(1, 1)))
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusReady)
	assert.Assert(t, !listed(p.Client.(*fake.Clientset), "services"))
}

// Missing RBAC on endpointslices falls back to the replica-based status, and
// warns once instead of on every request.
func TestInstanceInspectReadiness_FallsBackWhenForbidden(t *testing.T) {
	var logs bytes.Buffer
	p := endpointsProvider(webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels))
	p.l = slog.New(slog.NewTextHandler(&logs, nil))
	client := p.Client.(*fake.Clientset)
	client.PrependReactor("list", "endpointslices", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "discovery.k8s.io", Resource: "endpointslices"}, "", errors.New("RBAC"))
	})

	for range 2 {
		info, err := p.InstanceInspectReadiness(context.Background(), deploymentName(webDeployment(1, 1)))
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusReady)
		assert.Equal(t, info.Message, "")
	}
	assert.Assert(t, listed(client, "endpointslices"), "the endpointslices were never listed")
	assert.Equal(t, strings.Count(logs.String(), "level=WARN"), 1, logs.String())
}

// Any other failure, such as a cancelled request or a throttled API call, keeps
// the workload starting: a ready status is stored in the session and never
// checked again, so falling back would release requests before the Service
// routes to the workload.
func TestInstanceInspectReadiness_StaysStartingOnTransientError(t *testing.T) {
	p := endpointsProvider(webDeployment(1, 1), webPod("web-1", "u1"), service("web", webLabels))
	p.Client.(*fake.Clientset).PrependReactor("list", "endpointslices", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewTooManyRequests("slow down", 1)
	})

	info, err := p.InstanceInspectReadiness(context.Background(), deploymentName(webDeployment(1, 1)))
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusStarting)
	assert.Equal(t, info.Message, "cannot check whether the services route to the workload: list endpointslices: slow down")
}

func TestInstanceInspectReadiness_StatefulSet(t *testing.T) {
	dbLabels := map[string]string{"app": "db"}
	one := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns", UID: "db-uid"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: dbLabels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: dbLabels}},
		},
		Status: appsv1.StatefulSetStatus{Replicas: 1, ReadyReplicas: 1},
	}
	// The StatefulSet controller adds the pod-name label, which is not in the
	// pod template.
	db0 := readyPod("db-0", "u1", map[string]string{"app": "db", "statefulset.kubernetes.io/pod-name": "db-0"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "db", UID: "db-uid", Controller: ptr(true)})
	name := StatefulSetName(ss, ParseOptions{Delimiter: "_"}).Original

	tests := []struct {
		name        string
		objects     []runtime.Object
		want        sablier.InstanceStatus
		wantMessage string
	}{
		{
			name: "not routed yet",
			objects: []runtime.Object{ss, db0, service("db", dbLabels),
				endpointSlice("db-a", "db", endpoint("db-0", "u1", ptr(false)))},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "db" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "routed",
			objects: []runtime.Object{ss, db0, service("db", dbLabels),
				endpointSlice("db-a", "db", endpoint("db-0", "u1", ptr(true)))},
			want: sablier.InstanceStatusReady,
		},
		{
			name: "a per-replica service selects a label the controller adds",
			objects: []runtime.Object{ss, db0,
				service("db", dbLabels), service("db-0", map[string]string{"statefulset.kubernetes.io/pod-name": "db-0"}),
				endpointSlice("db-a", "db", endpoint("db-0", "u1", ptr(true))),
				endpointSlice("db-0-a", "db-0", endpoint("db-0", "u1", ptr(false)))},
			want:        sablier.InstanceStatusStarting,
			wantMessage: `waiting for service "db-0" to route to the workload: 0 of 1 pods routed`,
		},
		{
			name: "a pod of another statefulset is not counted",
			objects: []runtime.Object{ss,
				readyPod("db-0", "u1", dbLabels, metav1.OwnerReference{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "db", UID: "former-db-uid", Controller: ptr(true)}),
				service("db", dbLabels),
				endpointSlice("db-a", "db", endpoint("db-0", "u1", ptr(true)))},
			want:        sablier.InstanceStatusStarting,
			wantMessage: "waiting for the pods of the workload: 0 of 1 ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := endpointsProvider(tt.objects...)

			info, err := p.InstanceInspectReadiness(context.Background(), name)
			assert.NilError(t, err)
			assert.Equal(t, info.Status, tt.want)
			assert.Equal(t, info.Message, tt.wantMessage)
		})
	}
}
