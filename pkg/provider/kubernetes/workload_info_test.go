package kubernetes

import (
	"context"
	"log/slog"
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeploymentInspect_KubernetesWorkloadInfo(t *testing.T) {
	replicas := int32(0)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-app", Namespace: "my-ns", Labels: map[string]string{"a": "b"}},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}
	p := &Provider{Client: fake.NewSimpleClientset(d), delimiter: "_", l: slog.New(slog.DiscardHandler)}

	parsed := DeploymentName(d, ParseOptions{Delimiter: "_"})
	info, err := p.DeploymentInspect(context.Background(), parsed)
	assert.NilError(t, err)

	// The instance name format is the public API and must not change.
	assert.Equal(t, info.Name, "deployment_my-ns_my-app_1")
	assert.DeepEqual(t, info.Kubernetes, &sablier.KubernetesWorkloadInfo{
		Namespace: "my-ns",
		Kind:      "deployment",
		Name:      "my-app",
		Labels:    map[string]string{"a": "b"},
	})
}

func TestStatefulSetInspect_KubernetesWorkloadInfo(t *testing.T) {
	replicas := int32(0)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-db", Namespace: "my-ns"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
	}
	p := &Provider{Client: fake.NewSimpleClientset(ss), delimiter: "_", l: slog.New(slog.DiscardHandler)}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: "_"})
	info, err := p.StatefulSetInspect(context.Background(), parsed)
	assert.NilError(t, err)

	assert.Equal(t, info.Name, "statefulset_my-ns_my-db_1")
	assert.DeepEqual(t, info.Kubernetes, &sablier.KubernetesWorkloadInfo{
		Namespace: "my-ns",
		Kind:      "statefulset",
		Name:      "my-db",
		Labels:    map[string]string{},
	})
}
