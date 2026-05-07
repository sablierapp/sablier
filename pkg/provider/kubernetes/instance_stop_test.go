package kubernetes_test

import (
	"context"
	"fmt"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	"testing"
)

func TestKubernetesProvider_InstanceStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(kind *kindContainer) (string, error)
	}
	tests := []struct {
		name            string
		args            args
		ignoreUnlabeled bool
		err             error
	}{
		{
			name: "invalid name returns parser error",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "invalid-name", nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("invalid name [invalid-name] should be: kind_namespace_name_replicas"),
		},
		{
			name: "non-existing deployment stop returns provider error",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "deployment_default_my-deployment_1", nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("\"my-deployment\" not found"),
		},
		{
			name: "unlabeled deployment stop is rejected when ignoreUnlabeled is enabled",
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
			ignoreUnlabeled: true,
			err:             fmt.Errorf("is not managed by sablier"),
		},
		{
			name: "unlabeled deployment stop succeeds when ignoreUnlabeled is disabled",
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
			ignoreUnlabeled: false,
			err:             nil,
		},
		{
			name: "labeled deployment stop succeeds when ignoreUnlabeled is enabled",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					d, err := kind.CreateMimicDeployment(ctx, MimicOptions{Labels: map[string]string{"sablier.enable": "true"}})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentReady(ctx, kind.client, d.Namespace, d.Name); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			ignoreUnlabeled: true,
			err:             nil,
		},
		{
			name: "non-existing statefulSet stop returns provider error",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					return "statefulset_default_my-statefulset_1", nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("statefulsets.apps \"my-statefulset\" not found"),
		},
		{
			name: "unlabeled statefulSet stop is rejected when ignoreUnlabeled is enabled",
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
			ignoreUnlabeled: true,
			err:             fmt.Errorf("is not managed by sablier"),
		},
		{
			name: "unlabeled statefulSet stop succeeds when ignoreUnlabeled is disabled",
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
			ignoreUnlabeled: false,
			err:             nil,
		},
		{
			name: "labeled statefulSet stop succeeds when ignoreUnlabeled is enabled",
			args: args{
				do: func(kind *kindContainer) (string, error) {
					ss, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{Labels: map[string]string{"sablier.enable": "true"}})
					if err != nil {
						return "", err
					}

					if err = WaitForStatefulSetReady(ctx, kind.client, ss.Namespace, ss.Name); err != nil {
						return "", fmt.Errorf("error waiting for statefulSet: %w", err)
					}

					return kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			ignoreUnlabeled: true,
			err:             nil,
		},
	}
	kind := setupKinD(t, ctx)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := kubernetes.New(ctx, kind.client, slogt.New(t), config.NewProviderConfig().Kubernetes, tt.ignoreUnlabeled)
			assert.NilError(t, err)

			name, err := tt.args.do(kind)
			assert.NilError(t, err)

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)

				status, err := p.InstanceInspect(t.Context(), name)
				assert.NilError(t, err)

				assert.Equal(t, status.IsReady(), false)
			}
		})
	}
}
