package kubernetes_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	kind := sharedKinD
	p, err := kubernetes.New(ctx, kind.client, kind.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)

	d1, err := kind.CreateMimicDeployment(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().Deployments(d1.Namespace).Delete(context.Background(), d1.Name, metav1.DeleteOptions{})
	})

	d2, err := kind.CreateMimicDeployment(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().Deployments(d2.Namespace).Delete(context.Background(), d2.Name, metav1.DeleteOptions{})
	})

	ss1, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().StatefulSets(ss1.Namespace).Delete(context.Background(), ss1.Name, metav1.DeleteOptions{})
	})

	ss2, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().StatefulSets(ss2.Namespace).Delete(context.Background(), ss2.Name, metav1.DeleteOptions{})
	})

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
		{
			Name:    kubernetes.DeploymentName(d1, kubernetes.ParseOptions{Delimiter: "_"}).Original,
			Groups:  []string{"default"},
			Enabled: "true",
		},
		{
			Name:    kubernetes.DeploymentName(d2, kubernetes.ParseOptions{Delimiter: "_"}).Original,
			Groups:  []string{"my-group"},
			Enabled: "true",
		},
		{
			Name:    kubernetes.StatefulSetName(ss1, kubernetes.ParseOptions{Delimiter: "_"}).Original,
			Groups:  []string{"default"},
			Enabled: "true",
		},
		{
			Name:    kubernetes.StatefulSetName(ss2, kubernetes.ParseOptions{Delimiter: "_"}).Original,
			Groups:  []string{"my-group"},
			Enabled: "true",
		},
	}
	// Assert go is equal to want
	// Sort both array to ensure they are equal
	sort.Slice(got, func(i, j int) bool {
		return strings.Compare(got[i].Name, got[j].Name) < 0
	})
	sort.Slice(want, func(i, j int) bool {
		return strings.Compare(want[i].Name, want[j].Name) < 0
	})
	assert.DeepEqual(t, got, want)
}

func TestKubernetesProvider_InstanceGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	kind := sharedKinD
	p, err := kubernetes.New(ctx, kind.client, kind.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)

	d1, err := kind.CreateMimicDeployment(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().Deployments(d1.Namespace).Delete(context.Background(), d1.Name, metav1.DeleteOptions{})
	})

	d2, err := kind.CreateMimicDeployment(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group-1",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().Deployments(d2.Namespace).Delete(context.Background(), d2.Name, metav1.DeleteOptions{})
	})

	ss1, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().StatefulSets(ss1.Namespace).Delete(context.Background(), ss1.Name, metav1.DeleteOptions{})
	})

	ss2, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group-2",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().StatefulSets(ss2.Namespace).Delete(context.Background(), ss2.Name, metav1.DeleteOptions{})
	})

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default": {
			kubernetes.DeploymentName(d1, kubernetes.ParseOptions{Delimiter: "_"}).Original,
			kubernetes.StatefulSetName(ss1, kubernetes.ParseOptions{Delimiter: "_"}).Original,
		},
		"my-group-1": {
			kubernetes.DeploymentName(d2, kubernetes.ParseOptions{Delimiter: "_"}).Original,
		},
		"my-group-2": {
			kubernetes.StatefulSetName(ss2, kubernetes.ParseOptions{Delimiter: "_"}).Original,
		},
	}
	assert.DeepEqual(t, got, want)
}

// TestKubernetesProvider_InstanceGroups_Annotations verifies the real-world
// motivation for annotation support: workloads are discovered through the
// server-side "sablier.enable=true" label selector, while "sablier.group" —
// which here holds a comma-separated value that is invalid as a Kubernetes
// label — is read from annotations. It also asserts that an annotation takes
// precedence over a label for the same key.
func TestKubernetesProvider_InstanceGroups_Annotations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	kind := sharedKinD
	p, err := kubernetes.New(ctx, kind.client, kind.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)

	// enable is a label (required for discovery), group is a comma-separated
	// annotation — impossible to express as a single label value.
	d1, err := kind.CreateMimicDeployment(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
		Annotations: map[string]string{
			"sablier.group": "annotated-a,annotated-b",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().Deployments(d1.Namespace).Delete(context.Background(), d1.Name, metav1.DeleteOptions{})
	})

	// The annotation value must win over the label value for the same key.
	ss1, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "from-label",
		},
		Annotations: map[string]string{
			"sablier.group": "from-annotation",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = kind.client.AppsV1().StatefulSets(ss1.Namespace).Delete(context.Background(), ss1.Name, metav1.DeleteOptions{})
	})

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	dName := kubernetes.DeploymentName(d1, kubernetes.ParseOptions{Delimiter: "_"}).Original
	ssName := kubernetes.StatefulSetName(ss1, kubernetes.ParseOptions{Delimiter: "_"}).Original

	// InstanceGroups returns a cluster-wide view, so assert only on the groups
	// uniquely created by this test rather than the entire map.
	assert.DeepEqual(t, got["annotated-a"], []string{dName})
	assert.DeepEqual(t, got["annotated-b"], []string{dName})
	assert.DeepEqual(t, got["from-annotation"], []string{ssName})
	// The label value must not survive once overridden by the annotation.
	_, hasLabelGroup := got["from-label"]
	assert.Assert(t, !hasLabelGroup, "annotation should override label; group 'from-label' must not appear")
}
