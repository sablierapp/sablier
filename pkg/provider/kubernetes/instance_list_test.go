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
