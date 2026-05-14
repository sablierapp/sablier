package kubernetes_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesProvider_InstanceStart_ScaleMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	kind := sharedKinD
	p, err := kubernetes.New(ctx, kind.client, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)

	t.Run("deployment scale mode active resources applied", func(t *testing.T) {
		t.Parallel()

		d, err := kind.CreateMimicDeployment(ctx, MimicOptions{
			Labels: map[string]string{
				"sablier.active.cpu":    "200m",
				"sablier.active.memory": "128Mi",
			},
		})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = kind.client.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{})
		})

		err = WaitForDeploymentReady(ctx, kind.client, d.Namespace, d.Name)
		assert.NilError(t, err)

		name := kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original
		err = p.InstanceStart(ctx, name)
		assert.NilError(t, err)

		updated, err := kind.client.AppsV1().Deployments(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		limits := updated.Spec.Template.Spec.Containers[0].Resources.Limits
		expectedCPU := resource.MustParse("200m")
		expectedMem := resource.MustParse("128Mi")
		cpuLimit := limits[corev1.ResourceCPU]
		memLimit := limits[corev1.ResourceMemory]
		assert.Assert(t, cpuLimit.Cmp(expectedCPU) == 0,
			"expected CPU limit 200m, got %s", cpuLimit.String())
		assert.Assert(t, memLimit.Cmp(expectedMem) == 0,
			"expected memory limit 128Mi, got %s", memLimit.String())
	})

	t.Run("statefulset scale mode active resources applied", func(t *testing.T) {
		t.Parallel()

		ss, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{
			Labels: map[string]string{
				"sablier.active.cpu":    "200m",
				"sablier.active.memory": "128Mi",
			},
		})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = kind.client.AppsV1().StatefulSets(ss.Namespace).Delete(context.Background(), ss.Name, metav1.DeleteOptions{})
		})

		err = WaitForStatefulSetReady(ctx, kind.client, ss.Namespace, ss.Name)
		assert.NilError(t, err)

		name := kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original
		err = p.InstanceStart(ctx, name)
		assert.NilError(t, err)

		updated, err := kind.client.AppsV1().StatefulSets(ss.Namespace).Get(ctx, ss.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		limits := updated.Spec.Template.Spec.Containers[0].Resources.Limits
		expectedCPU := resource.MustParse("200m")
		expectedMem := resource.MustParse("128Mi")
		cpuLimit := limits[corev1.ResourceCPU]
		memLimit := limits[corev1.ResourceMemory]
		assert.Assert(t, cpuLimit.Cmp(expectedCPU) == 0,
			"expected CPU limit 200m, got %s", cpuLimit.String())
		assert.Assert(t, memLimit.Cmp(expectedMem) == 0,
			"expected memory limit 128Mi, got %s", memLimit.String())
	})
}

func TestKubernetesProvider_InstanceStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	type args struct {
		do func(kind *kindContainer) (string, error)
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "invalid name",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "invalid-name", nil
				},
			},
			err: fmt.Errorf("invalid name [invalid-name] should be: kind_namespace_name_replicas"),
		},
		{
			name: "non existing deployment start",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "deployment_default_my-deployment_1", nil
				},
			},
			err: fmt.Errorf("deployments.apps \"my-deployment\" not found"),
		},
		{
			name: "deployment start as expected",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					d, err := kind.CreateMimicDeployment(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentReady(ctx, kind.client, d.Namespace, d.Name); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			err: nil,
		},
		{
			name: "non existing statefulSet start",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "statefulset_default_my-statefulset_1", nil
				},
			},
			err: fmt.Errorf("statefulsets.apps \"my-statefulset\" not found"),
		},
		{
			name: "statefulSet start as expected",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					ss, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForStatefulSetReady(ctx, kind.client, ss.Namespace, ss.Name); err != nil {
						return "", fmt.Errorf("error waiting for statefulSet: %w", err)
					}

					return kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			err: nil,
		},
	}
	kind := sharedKinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := kubernetes.New(ctx, kind.client, slogt.New(t), config.NewProviderConfig().Kubernetes)
			assert.NilError(t, err)

			name, err := tt.args.do(kind)
			assert.NilError(t, err)

			// Clean up the workload created by this subtest.
			if parsed, parseErr := kubernetes.ParseName(name, kubernetes.ParseOptions{Delimiter: "_"}); parseErr == nil {
				t.Cleanup(func() {
					switch parsed.Kind {
					case "deployment":
						_ = kind.client.AppsV1().Deployments(parsed.Namespace).Delete(context.Background(), parsed.Name, metav1.DeleteOptions{})
					case "statefulset":
						_ = kind.client.AppsV1().StatefulSets(parsed.Namespace).Delete(context.Background(), parsed.Name, metav1.DeleteOptions{})
					}
				})
			}

			err = p.InstanceStart(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)

				status, err := p.InstanceInspect(t.Context(), name)
				assert.NilError(t, err)

				assert.Equal(t, status.IsReady(), true)
			}
		})
	}
}
