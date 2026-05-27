package kubernetes_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	crdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	cnpgGVR = schema.GroupVersionResource{
		Group:    "postgresql.cnpg.io",
		Version:  "v1",
		Resource: "clusters",
	}
	cnpgCRDOnce sync.Once
)

// ensureCNPGClusterCRD installs a minimal CloudNativePG Cluster CRD into the shared
// cluster (once) so the real API server accepts Cluster resources. It intentionally
// does not run the CloudNativePG operator: these tests verify Sablier's interactions
// with the Cluster API (hibernation annotation, listing, inspection), not Postgres.
func ensureCNPGClusterCRD(ctx context.Context, t *testing.T, kind *kindContainer) {
	t.Helper()
	cnpgCRDOnce.Do(func() {
		crd := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": "clusters.postgresql.cnpg.io"},
			"spec": map[string]any{
				"group": "postgresql.cnpg.io",
				"scope": "Namespaced",
				"names": map[string]any{
					"plural":   "clusters",
					"singular": "cluster",
					"kind":     "Cluster",
					"listKind": "ClusterList",
				},
				"versions": []any{
					map[string]any{
						"name":    "v1",
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

		_, err := kind.dynamic.Resource(crdGVR).Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create cnpg cluster CRD: %v", err)
		}

		// Wait until the API server serves the new resource type.
		deadline := time.Now().Add(30 * time.Second)
		for {
			_, err := kind.dynamic.Resource(cnpgGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 1})
			if err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("cnpg cluster CRD not established in time: %v", err)
			}
			time.Sleep(200 * time.Millisecond)
		}
	})
}

func createCNPGCluster(ctx context.Context, t *testing.T, kind *kindContainer, name string, labels map[string]string, instances, readyInstances int64) {
	t.Helper()
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"labels":    toStringMap(labels),
		},
		"spec": map[string]any{
			"instances": instances,
			"imageName": "ghcr.io/cloudnative-pg/postgresql:16",
		},
		"status": map[string]any{
			"readyInstances": readyInstances,
		},
	}}

	_, err := kind.dynamic.Resource(cnpgGVR).Namespace("default").Create(ctx, cluster, metav1.CreateOptions{})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.dynamic.Resource(cnpgGVR).Namespace("default").Delete(context.Background(), name, metav1.DeleteOptions{})
	})
}

func toStringMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func clusterHibernationAnnotation(ctx context.Context, t *testing.T, kind *kindContainer, name string) string {
	t.Helper()
	u, err := kind.dynamic.Resource(cnpgGVR).Namespace("default").Get(ctx, name, metav1.GetOptions{})
	assert.NilError(t, err)
	return u.GetAnnotations()["cnpg.io/hibernation"]
}

func TestKubernetesProvider_CNPGCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	kind := sharedKinD
	ensureCNPGClusterCRD(ctx, t, kind)

	p, err := kubernetes.New(ctx, kind.client, kind.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)

	name := "pg-" + generateRandomName()
	createCNPGCluster(ctx, t, kind, name, map[string]string{
		"sablier.enable": "true",
		"sablier.group":  "opencell",
	}, 1, 1)

	instanceName := kubernetes.ClusterName("default", name, kubernetes.ParseOptions{Delimiter: "_"}).Original

	t.Run("inspect reports ready", func(t *testing.T) {
		info, err := p.InstanceInspect(ctx, instanceName)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusReady)
		assert.Equal(t, info.Kubernetes.Kind, kubernetes.KindCNPGCluster)
	})

	t.Run("stop hibernates the cluster", func(t *testing.T) {
		assert.NilError(t, p.InstanceStop(ctx, instanceName))
		assert.Equal(t, clusterHibernationAnnotation(ctx, t, kind, name), "on")

		info, err := p.InstanceInspect(ctx, instanceName)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
	})

	t.Run("start resumes the cluster", func(t *testing.T) {
		assert.NilError(t, p.InstanceStart(ctx, instanceName))
		assert.Equal(t, clusterHibernationAnnotation(ctx, t, kind, name), "off")
	})

	t.Run("list discovers the cluster", func(t *testing.T) {
		instances, err := p.ClusterList(ctx)
		assert.NilError(t, err)
		found := false
		for _, i := range instances {
			if i.Name == instanceName {
				found = true
				assert.DeepEqual(t, i.Groups, []string{"opencell"})
			}
		}
		assert.Assert(t, found, fmt.Sprintf("expected to find %s in cluster list", instanceName))
	})
}
