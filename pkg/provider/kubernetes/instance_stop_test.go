package kubernetes_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesProvider_InstanceStop(t *testing.T) {
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
			name: "non existing deployment stop",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "deployment_default_my-deployment_1", nil
				},
			},
			err: fmt.Errorf("deployments.apps \"my-deployment\" not found"),
		},
		{
			name: "deployment stop as expected",
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
			name: "non existing statefulSet stop",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "statefulset_default_my-statefulset_1", nil
				},
			},
			err: fmt.Errorf("statefulsets.apps \"my-statefulset\" not found"),
		},
		{
			name: "statefulSet stop as expected",
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

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)

				status, err := p.InstanceInspect(t.Context(), name)
				assert.NilError(t, err)

				assert.Equal(t, status.IsReady(), false)
			}
		})
	}
}
