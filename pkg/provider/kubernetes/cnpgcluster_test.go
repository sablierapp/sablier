package kubernetes

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.opentelemetry.io/otel"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

// newClusterObj builds an unstructured CloudNativePG Cluster for tests.
func newClusterObj(namespace, name string, labels, annotations map[string]string, instances, readyInstances int64) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("postgresql.cnpg.io/v1")
	u.SetKind("Cluster")
	u.SetNamespace(namespace)
	u.SetName(name)
	if labels != nil {
		u.SetLabels(labels)
	}
	if annotations != nil {
		u.SetAnnotations(annotations)
	}
	_ = unstructured.SetNestedField(u.Object, instances, "spec", "instances")
	_ = unstructured.SetNestedField(u.Object, "ghcr.io/cloudnative-pg/postgresql:16", "spec", "imageName")
	_ = unstructured.SetNestedField(u.Object, readyInstances, "status", "readyInstances")
	return u
}

func newFakeCNPGProvider(t *testing.T, objs ...runtime.Object) *Provider {
	t.Helper()
	scheme := runtime.NewScheme()
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{cnpgClusterGVR: "ClusterList"},
		objs...,
	)
	return &Provider{
		dynamic:   dyn,
		delimiter: "_",
		l:         slogt.New(t),
		tracer:    otel.Tracer("test"),
	}
}

func TestProvider_ClusterInspect(t *testing.T) {
	t.Parallel()

	enabled := map[string]string{"sablier.enable": "true", "sablier.group": "opencell"}

	tests := []struct {
		name           string
		labels         map[string]string
		annotations    map[string]string
		instances      int64
		readyInstances int64
		wantStatus     sablier.InstanceStatus
	}{
		{
			name:           "ready when all instances are ready",
			labels:         enabled,
			instances:      3,
			readyInstances: 3,
			wantStatus:     sablier.InstanceStatusReady,
		},
		{
			name:           "starting when not all instances are ready",
			labels:         enabled,
			instances:      3,
			readyInstances: 1,
			wantStatus:     sablier.InstanceStatusStarting,
		},
		{
			name:           "stopped when hibernation annotation is on",
			labels:         enabled,
			annotations:    map[string]string{cnpgHibernationAnnotation: cnpgHibernationOn},
			instances:      3,
			readyInstances: 0,
			wantStatus:     sablier.InstanceStatusStopped,
		},
		{
			name:           "hibernation on takes precedence over ready instances",
			labels:         enabled,
			annotations:    map[string]string{cnpgHibernationAnnotation: cnpgHibernationOn},
			instances:      1,
			readyInstances: 1,
			wantStatus:     sablier.InstanceStatusStopped,
		},
		{
			name:           "ready with defaulted single instance",
			labels:         enabled,
			instances:      0, // unset -> defaults to 1
			readyInstances: 1,
			wantStatus:     sablier.InstanceStatusReady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			obj := newClusterObj("default", "pg", tc.labels, tc.annotations, tc.instances, tc.readyInstances)
			p := newFakeCNPGProvider(t, obj)

			parsed := ClusterName("default", "pg", ParseOptions{Delimiter: "_"})
			info, err := p.ClusterInspect(context.Background(), parsed)
			assert.NilError(t, err)
			assert.Equal(t, info.Status, tc.wantStatus)
			assert.Equal(t, info.Provider, sablier.ProviderKubernetes)
			assert.Assert(t, info.Kubernetes != nil)
			assert.Equal(t, info.Kubernetes.Kind, KindCNPGCluster)
			assert.Equal(t, info.Kubernetes.Image, "ghcr.io/cloudnative-pg/postgresql:16")
		})
	}
}

func TestProvider_ClusterHibernate(t *testing.T) {
	t.Parallel()

	getAnnotation := func(t *testing.T, p *Provider) string {
		t.Helper()
		u, err := p.dynamic.Resource(cnpgClusterGVR).Namespace("default").Get(context.Background(), "pg", metav1.GetOptions{})
		assert.NilError(t, err)
		return u.GetAnnotations()[cnpgHibernationAnnotation]
	}

	t.Run("stop sets hibernation on", func(t *testing.T) {
		t.Parallel()
		obj := newClusterObj("default", "pg", nil, nil, 1, 1)
		p := newFakeCNPGProvider(t, obj)

		parsed := ClusterName("default", "pg", ParseOptions{Delimiter: "_"})
		assert.NilError(t, p.InstanceStop(context.Background(), parsed.Original))
		assert.Equal(t, getAnnotation(t, p), cnpgHibernationOn)
	})

	t.Run("start sets hibernation off", func(t *testing.T) {
		t.Parallel()
		obj := newClusterObj("default", "pg", nil, map[string]string{cnpgHibernationAnnotation: cnpgHibernationOn}, 1, 0)
		p := newFakeCNPGProvider(t, obj)

		parsed := ClusterName("default", "pg", ParseOptions{Delimiter: "_"})
		assert.NilError(t, p.InstanceStart(context.Background(), parsed.Original))
		assert.Equal(t, getAnnotation(t, p), cnpgHibernationOff)
	})
}

func TestProvider_ClusterListAndGroups(t *testing.T) {
	t.Parallel()

	enabled := newClusterObj("default", "pg-opencell",
		map[string]string{"sablier.enable": "true", "sablier.group": "opencell"}, nil, 1, 1)
	enabledDefaultGroup := newClusterObj("default", "pg-solo",
		map[string]string{"sablier.enable": "true"}, nil, 1, 1)
	disabled := newClusterObj("default", "pg-ignored", nil, nil, 1, 1)

	p := newFakeCNPGProvider(t, enabled, enabledDefaultGroup, disabled)

	instances, err := p.ClusterList(context.Background())
	assert.NilError(t, err)
	// Only the two sablier.enable=true clusters are listed.
	assert.Equal(t, len(instances), 2)

	groups, err := p.ClusterGroups(context.Background())
	assert.NilError(t, err)

	opencell := ClusterName("default", "pg-opencell", ParseOptions{Delimiter: "_"}).Original
	solo := ClusterName("default", "pg-solo", ParseOptions{Delimiter: "_"}).Original
	assert.DeepEqual(t, groups["opencell"], []string{opencell})
	assert.DeepEqual(t, groups["default"], []string{solo})
}
