package kubernetes_test

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var managedLabels = map[string]string{"sablier.enable": "true"}

func TestKubernetesProvider_IgnoreUnlabeled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	p, err := kubernetes.New(ctx, sharedKinD.client, slogt.New(t), config.NewProviderConfig().Kubernetes, true)
	assert.NilError(t, err)

	t.Run("unlabeled deployment start and stop are rejected", func(t *testing.T) {
		d, err := sharedKinD.CreateMimicDeployment(ctx, MimicOptions{})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = sharedKinD.client.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{})
		})

		name := kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original
		err = p.InstanceStart(ctx, name)
		assert.ErrorContains(t, err, "is not managed by sablier")

		err = p.InstanceStop(ctx, name)
		assert.ErrorContains(t, err, "is not managed by sablier")
	})

	t.Run("unlabeled statefulset start and stop are rejected", func(t *testing.T) {
		ss, err := sharedKinD.CreateMimicStatefulSet(ctx, MimicOptions{})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = sharedKinD.client.AppsV1().StatefulSets(ss.Namespace).Delete(context.Background(), ss.Name, metav1.DeleteOptions{})
		})

		name := kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original
		err = p.InstanceStart(ctx, name)
		assert.ErrorContains(t, err, "is not managed by sablier")

		err = p.InstanceStop(ctx, name)
		assert.ErrorContains(t, err, "is not managed by sablier")
	})

	t.Run("labeled deployment start and stop succeed", func(t *testing.T) {
		d, err := sharedKinD.CreateMimicDeployment(ctx, MimicOptions{Labels: managedLabels})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = sharedKinD.client.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{})
		})
		err = WaitForDeploymentReady(ctx, sharedKinD.client, d.Namespace, d.Name)
		assert.NilError(t, err)

		name := kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original
		err = p.InstanceStart(ctx, name)
		assert.NilError(t, err)

		err = p.InstanceStop(ctx, name)
		assert.NilError(t, err)
	})
}
