package kubernetes_test

import (
	"context"
	"fmt"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	"testing"
)

func TestKubernetesProvider_InstanceStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

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
			err: fmt.Errorf("deployments/scale.apps \"my-deployment\" not found"),
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
	kind := setupKinD(t, ctx)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := kubernetes.New(ctx, kind.client, slogt.New(t), config.NewProviderConfig().Kubernetes)
			assert.NilError(t, err)

			name, err := tt.args.do(kind)
			assert.NilError(t, err)

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
