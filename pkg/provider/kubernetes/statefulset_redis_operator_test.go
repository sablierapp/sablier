package kubernetes

import (
	"context"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"go.opentelemetry.io/otel"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newRedisOwnerSTS builds a StatefulSet whose controller owner is a Redis CR.
func newRedisOwnerSTS(namespace, name string, replicas int32, apiVersion string) *appsv1.StatefulSet {
	if apiVersion == "" {
		apiVersion = redisOperatorAPIVersion()
	}
	isController := true
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"sablier.enable": "true"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: apiVersion,
				Kind:       "Redis",
				Name:       name, // Redis CR has same name as the StatefulSet by convention
				Controller: &isController,
			}},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: replicas},
	}
}

// redisOperatorAPIVersion returns the canonical APIVersion string for the operator.
func redisOperatorAPIVersion() string {
	return redisOperatorGVR.Group + "/v1beta2"
}

// newRedisOperatorTestProvider builds a Provider with both a fake typed client
// (for StatefulSet get/scale) and a fake dynamic client (for Redis CR annotation patch).
// The Redis CR is pre-created in the dynamic tracker under the operator GVR.
func newRedisOperatorTestProvider(t *testing.T, sts *appsv1.StatefulSet) (*Provider, *dynamicfake.FakeDynamicClient) {
	t.Helper()

	// Typed client — StatefulSet with Scale reactor.
	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}
	stsReplicas := map[string]int32{sts.Name: replicas}
	client := k8sfake.NewSimpleClientset(sts)
	client.PrependReactor("get", "statefulsets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		name := action.(k8stesting.GetAction).GetName()
		return true, &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: action.GetNamespace()},
			Spec:       autoscalingv1.ScaleSpec{Replicas: stsReplicas[name]},
		}, nil
	})
	client.PrependReactor("update", "statefulsets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		s := action.(k8stesting.UpdateAction).GetObject().(*autoscalingv1.Scale)
		stsReplicas[s.Name] = s.Spec.Replicas
		return true, s, nil
	})

	// Dynamic client — pre-create the Redis CR so annotations can be patched.
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{redisOperatorGVR: "RedisList"},
	)
	redisObj := &unstructured.Unstructured{}
	redisObj.SetAPIVersion(redisOperatorAPIVersion())
	redisObj.SetKind("Redis")
	redisObj.SetNamespace(sts.Namespace)
	redisObj.SetName(sts.Name)
	if err := dyn.Tracker().Create(redisOperatorGVR, redisObj, sts.Namespace); err != nil {
		t.Fatalf("failed to pre-create Redis CR: %v", err)
	}

	p := &Provider{
		Client:    client,
		dynamic:   dyn,
		delimiter: "_",
		l:         slogt.New(t),
		tracer:    otel.Tracer("test"),
	}
	return p, dyn
}

// getRedisAnnotation reads the skip-reconcile annotation from the fake dynamic tracker.
func getRedisAnnotation(t *testing.T, dyn *dynamicfake.FakeDynamicClient, namespace, name string) string {
	t.Helper()
	u, err := dyn.Resource(redisOperatorGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	assert.NilError(t, err)
	return u.GetAnnotations()[redisOperatorSkipReconcileAnnotation]
}

// --- Unit tests for redisOperatorOwner ---

func TestRedisOperatorOwner(t *testing.T) {
	t.Parallel()

	isController := true
	notController := false

	tests := []struct {
		name      string
		refs      []metav1.OwnerReference
		wantName  string
		wantFound bool
	}{
		{
			name: "matched by group and kind",
			refs: []metav1.OwnerReference{{
				APIVersion: redisOperatorGVR.Group + "/v1beta2",
				Kind:       "Redis",
				Name:       "my-redis",
				Controller: &isController,
			}},
			wantName: "my-redis", wantFound: true,
		},
		{
			name: "matched even when version is v1 (forward-compat)",
			refs: []metav1.OwnerReference{{
				APIVersion: redisOperatorGVR.Group + "/v1",
				Kind:       "Redis",
				Name:       "my-redis",
				Controller: &isController,
			}},
			wantName: "my-redis", wantFound: true,
		},
		{
			name: "not matched when kind is wrong",
			refs: []metav1.OwnerReference{{
				APIVersion: redisOperatorGVR.Group + "/v1beta2",
				Kind:       "RedisCluster",
				Name:       "my-redis",
				Controller: &isController,
			}},
			wantFound: false,
		},
		{
			name: "not matched when group is wrong",
			refs: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "Redis",
				Name:       "my-redis",
				Controller: &isController,
			}},
			wantFound: false,
		},
		{
			name: "not matched when controller=false",
			refs: []metav1.OwnerReference{{
				APIVersion: redisOperatorGVR.Group + "/v1beta2",
				Kind:       "Redis",
				Name:       "my-redis",
				Controller: &notController,
			}},
			wantFound: false,
		},
		{
			name:      "not matched with no owner references",
			refs:      nil,
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ss := &appsv1.StatefulSet{}
			ss.OwnerReferences = tc.refs
			name, ok := redisOperatorOwner(ss)
			assert.Equal(t, ok, tc.wantFound)
			if tc.wantFound {
				assert.Equal(t, name, tc.wantName)
			}
		})
	}
}

// --- Unit tests for apiVersionGroup ---

func TestAPIVersionGroup(t *testing.T) {
	t.Parallel()
	assert.Equal(t, apiVersionGroup("apps/v1"), "apps")
	assert.Equal(t, apiVersionGroup("redis.redis.opstreelabs.in/v1beta2"), "redis.redis.opstreelabs.in")
	assert.Equal(t, apiVersionGroup("v1"), "v1") // core resource, no slash
}

// --- Unit tests for setRedisOperatorSkipReconcile ---

func TestSetRedisOperatorSkipReconcile_Set(t *testing.T) {
	t.Parallel()
	sts := newRedisOwnerSTS("default", "my-redis", 1, "")
	p, dyn := newRedisOperatorTestProvider(t, sts)

	p.setRedisOperatorSkipReconcile(context.Background(), sts, true)
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "true")
}

func TestSetRedisOperatorSkipReconcile_Clear(t *testing.T) {
	t.Parallel()
	sts := newRedisOwnerSTS("default", "my-redis", 1, "")
	p, dyn := newRedisOperatorTestProvider(t, sts)

	// Set then clear.
	p.setRedisOperatorSkipReconcile(context.Background(), sts, true)
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "true")

	p.setRedisOperatorSkipReconcile(context.Background(), sts, false)
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "")
}

func TestSetRedisOperatorSkipReconcile_NonRedisStatefulSet(t *testing.T) {
	t.Parallel()
	// Plain StatefulSet with no owner references — patch must not be called.
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "plain-sts", Namespace: "default"},
	}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{redisOperatorGVR: "RedisList"},
	)
	p := &Provider{
		Client:    k8sfake.NewSimpleClientset(sts),
		dynamic:   dyn,
		delimiter: "_",
		l:         slogt.New(t),
		tracer:    otel.Tracer("test"),
	}

	// Should be a no-op — no panic, no patch.
	p.setRedisOperatorSkipReconcile(context.Background(), sts, true)
	assert.Equal(t, len(dyn.Actions()), 0)
}

// --- Integration-style tests for InstanceStop annotation behaviour ---

func TestInstanceStop_SetsSkipReconcileBeforeScale(t *testing.T) {
	t.Parallel()
	sts := newRedisOwnerSTS("default", "my-redis", 1, "")
	p, dyn := newRedisOperatorTestProvider(t, sts)

	instanceName := StatefulSetName(sts, ParseOptions{Delimiter: "_"}).Original
	err := p.InstanceStop(context.Background(), instanceName)
	assert.NilError(t, err)

	// Annotation must be set.
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "true")

	// StatefulSet must be scaled to zero.
	s, scaleErr := p.Client.AppsV1().StatefulSets("default").GetScale(context.Background(), "my-redis", metav1.GetOptions{})
	assert.NilError(t, scaleErr)
	assert.Equal(t, s.Spec.Replicas, int32(0))
}

func TestInstanceStop_ClearsAnnotationOnScaleFailure(t *testing.T) {
	t.Parallel()
	sts := newRedisOwnerSTS("default", "my-redis", 1, "")
	p, dyn := newRedisOperatorTestProvider(t, sts)

	// Inject a scale failure.
	p.Client.(*k8sfake.Clientset).PrependReactor("update", "statefulsets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "scale" {
			return true, nil, fmt.Errorf("simulated scale failure")
		}
		return false, nil, nil
	})

	instanceName := StatefulSetName(sts, ParseOptions{Delimiter: "_"}).Original
	err := p.InstanceStop(context.Background(), instanceName)
	assert.ErrorContains(t, err, "simulated scale failure")

	// Cleanup defer must have cleared the annotation.
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "")
}

func TestInstanceStart_ClearsSkipReconcileAfterScale(t *testing.T) {
	t.Parallel()
	// Start with replicas=0 and annotation set (simulating stopped state).
	sts := newRedisOwnerSTS("default", "my-redis", 0, "")
	p, dyn := newRedisOperatorTestProvider(t, sts)

	// Pre-set the annotation as if a prior stop had run.
	p.setRedisOperatorSkipReconcile(context.Background(), sts, true)
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "true")

	instanceName := StatefulSetName(sts, ParseOptions{Delimiter: "_"}).Original
	err := p.InstanceStart(context.Background(), instanceName)
	assert.NilError(t, err)

	// Annotation must be cleared after successful start.
	assert.Equal(t, getRedisAnnotation(t, dyn, "default", "my-redis"), "")
}
