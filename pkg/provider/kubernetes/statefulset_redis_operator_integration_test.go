package kubernetes_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	redisCRDOnce sync.Once

	redisCRDGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	redisGVR = schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redis",
	}
)

// ensureRedisCRD installs a minimal Redis CRD into the shared cluster (once)
// so the API server accepts Redis resources. It intentionally does not run the
// redis-operator: these tests verify Sablier's interactions with the Redis API
// (skip-reconcile annotation, StatefulSet scaling), not Redis itself.
func ensureRedisCRD(ctx context.Context, t *testing.T, kind *kindContainer) {
	t.Helper()
	redisCRDOnce.Do(func() {
		crd := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": "redis.redis.redis.opstreelabs.in"},
			"spec": map[string]any{
				"group": "redis.redis.opstreelabs.in",
				"scope": "Namespaced",
				"names": map[string]any{
					"plural":   "redis",
					"singular": "redis",
					"kind":     "Redis",
					"listKind": "RedisList",
				},
				"versions": []any{
					map[string]any{
						"name":    "v1beta2",
						"served":  true,
						"storage": true,
						"schema": map[string]any{
							"openAPIV3Schema": map[string]any{
								"type":                                 "object",
								"x-kubernetes-preserve-unknown-fields": true,
							},
						},
					},
				},
			},
		}}

		_, err := kind.dynamic.Resource(redisCRDGVR).Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create Redis CRD: %v", err)
		}

		// Wait until the API server serves the new resource type.
		deadline := time.Now().Add(30 * time.Second)
		for {
			_, err := kind.dynamic.Resource(redisGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 1})
			if err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("Redis CRD not established in time: %v", err)
			}
			time.Sleep(200 * time.Millisecond)
		}
	})
}

// createRedisAndStatefulSet creates a Redis CR and a companion StatefulSet whose
// ownerReference points to that CR, simulating what the redis-operator would create.
// This lets tests verify Sablier's annotation and scaling logic without running
// the actual operator.
func createRedisAndStatefulSet(ctx context.Context, t *testing.T, kind *kindContainer, name string, labels map[string]string) {
	t.Helper()

	// Create the Redis CR.
	redis := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "redis.redis.opstreelabs.in/v1beta2",
		"kind":       "Redis",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"labels":    toStringMap(labels),
		},
	}}
	cr, err := kind.dynamic.Resource(redisGVR).Namespace("default").Create(ctx, redis, metav1.CreateOptions{})
	assert.NilError(t, err)

	// Create a StatefulSet that mirrors what the redis-operator would produce,
	// with an ownerReference back to the Redis CR so Sablier can detect it.
	isController := true
	one := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "redis.redis.opstreelabs.in/v1beta2",
				Kind:       "Redis",
				Name:       cr.GetName(),
				UID:        cr.GetUID(),
				Controller: &isController,
			}},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "redis",
						Image: "quay.io/opstree/redis:v7.0.12",
					}},
				},
			},
		},
	}
	_, err = kind.client.AppsV1().StatefulSets("default").Create(ctx, sts, metav1.CreateOptions{})
	assert.NilError(t, err)

	t.Cleanup(func() {
		_ = kind.dynamic.Resource(redisGVR).Namespace("default").Delete(context.Background(), name, metav1.DeleteOptions{})
		_ = kind.client.AppsV1().StatefulSets("default").Delete(context.Background(), name, metav1.DeleteOptions{})
	})
}

// redisAnnotation reads the skip-reconcile annotation from the Redis CR.
func redisAnnotation(ctx context.Context, t *testing.T, kind *kindContainer, name string) string {
	t.Helper()
	u, err := kind.dynamic.Resource(redisGVR).Namespace("default").Get(ctx, name, metav1.GetOptions{})
	assert.NilError(t, err)
	return u.GetAnnotations()["redis.opstreelabs.in/skip-reconcile"]
}

func TestKubernetesProvider_RedisOperator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis-operator integration test in short mode.")
	}

	ctx := t.Context()
	kind := sharedKinD
	ensureRedisCRD(ctx, t, kind)

	p, err := kubernetes.New(ctx, kind.client, kind.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)

	name := "redis-" + generateRandomName()
	createRedisAndStatefulSet(ctx, t, kind, name, map[string]string{
		"sablier.enable": "true",
		"sablier.group":  "myapp",
	})

	sts, err := kind.client.AppsV1().StatefulSets("default").Get(ctx, name, metav1.GetOptions{})
	assert.NilError(t, err)
	instanceName := kubernetes.StatefulSetName(sts, kubernetes.ParseOptions{Delimiter: "_"}).Original

	// Wait for the StatefulSet controller to have processed the new StatefulSet
	// (observedGeneration == generation). This avoids resource-version conflicts
	// when Sablier's UpdateScale call races with the controller's status update.
	deadline := time.Now().Add(10 * time.Second)
	for {
		sts, err = kind.client.AppsV1().StatefulSets("default").Get(ctx, name, metav1.GetOptions{})
		assert.NilError(t, err)
		if sts.Status.ObservedGeneration >= sts.Generation {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("StatefulSet controller did not observe the StatefulSet in time")
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Run("stop sets skip-reconcile annotation and scales to zero", func(t *testing.T) {
		assert.NilError(t, p.InstanceStop(ctx, instanceName))

		assert.Equal(t, redisAnnotation(ctx, t, kind, name), "true")

		sts, err := kind.client.AppsV1().StatefulSets("default").Get(ctx, name, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Equal(t, *sts.Spec.Replicas, int32(0))
	})

	t.Run("inspect reports stopped after stop", func(t *testing.T) {
		info, err := p.InstanceInspect(ctx, instanceName)
		assert.NilError(t, err)
		assert.Equal(t, info.Kubernetes.Kind, "statefulset")
	})

	t.Run("start clears skip-reconcile annotation and scales back up", func(t *testing.T) {
		assert.NilError(t, p.InstanceStart(ctx, instanceName))

		assert.Equal(t, redisAnnotation(ctx, t, kind, name), "",
			"skip-reconcile annotation should be cleared after start")

		sts, err := kind.client.AppsV1().StatefulSets("default").Get(ctx, name, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Equal(t, *sts.Spec.Replicas, int32(1))
	})
}
