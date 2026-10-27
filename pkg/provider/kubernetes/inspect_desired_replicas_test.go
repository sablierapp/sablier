package kubernetes

// Unit tests for the desired replicas reported by the workload inspect paths.
// These run without a real Kubernetes cluster.

import (
	"context"
	"log/slog"
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInspect_DesiredReplicas(t *testing.T) {
	opts := ParseOptions{Delimiter: "_"}

	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		// nameReplicas, when set, overrides the replica count of the instance name.
		nameReplicas string
		want         int32
	}{
		{
			name:   "label set to 3",
			labels: map[string]string{"sablier.active.replicas": "3"},
			want:   3,
		},
		{
			name:   "public label set to 3",
			labels: map[string]string{"sablierapp.dev/active.replicas": "3"},
			want:   3,
		},
		{
			name: "label absent",
			want: 1,
		},
		{
			name:         "name with explicit count and no label",
			nameReplicas: "2",
			want:         2,
		},
		{
			name:         "label wins over name count",
			labels:       map[string]string{"sablier.active.replicas": "3"},
			nameReplicas: "2",
			want:         3,
		},
		{
			name:        "annotation instead of label",
			annotations: map[string]string{"sablier.active.replicas": "3"},
			want:        3,
		},
	}

	for _, tt := range tests {
		t.Run("deployment/"+tt.name, func(t *testing.T) {
			one := int32(1)
			d := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns", Labels: tt.labels, Annotations: tt.annotations},
				Spec:       appsv1.DeploymentSpec{Replicas: &one},
			}
			p := &Provider{Client: fake.NewSimpleClientset(d), delimiter: "_", l: slog.Default()}

			parsed := DeploymentName(d, opts)
			if tt.nameReplicas != "" {
				var err error
				parsed, err = ParseName("deployment_ns_app_"+tt.nameReplicas, opts)
				assert.NilError(t, err)
			}

			info, err := p.DeploymentInspect(context.Background(), parsed)
			assert.NilError(t, err)
			assert.Equal(t, info.DesiredReplicas, tt.want)
			assert.Equal(t, info.Name, parsed.Original)
		})

		t.Run("statefulset/"+tt.name, func(t *testing.T) {
			one := int32(1)
			ss := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns", Labels: tt.labels, Annotations: tt.annotations},
				Spec:       appsv1.StatefulSetSpec{Replicas: &one},
			}
			p := &Provider{Client: fake.NewSimpleClientset(ss), delimiter: "_", l: slog.Default()}

			parsed := StatefulSetName(ss, opts)
			if tt.nameReplicas != "" {
				var err error
				parsed, err = ParseName("statefulset_ns_app_"+tt.nameReplicas, opts)
				assert.NilError(t, err)
			}

			info, err := p.StatefulSetInspect(context.Background(), parsed)
			assert.NilError(t, err)
			assert.Equal(t, info.DesiredReplicas, tt.want)
			assert.Equal(t, info.Name, parsed.Original)
		})
	}
}

func TestDesiredReplicas_MatchesStart(t *testing.T) {
	parsed := ParsedName{Kind: "deployment", Namespace: "ns", Name: "app", Replicas: 1}
	cfg := sablierConfig(nil, map[string]string{"sablier.active.replicas": "4"})
	assert.Equal(t, desiredReplicas(cfg, parsed), int32(4))
}
