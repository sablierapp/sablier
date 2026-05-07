package docker_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_Stop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) (string, error)
	}
	tests := []struct {
		name            string
		args            args
		ignoreUnlabeled bool
		err             error
	}{
		{
			name: "non-existing container stop returns provider error",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("No such container: non-existent"),
		},
		{
			name: "unlabeled container stop is rejected when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("is not managed by sablier"),
		},
		{
			name: "unlabeled container stop succeeds when ignoreUnlabeled is disabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			ignoreUnlabeled: false,
			err:             nil,
		},
		{
			name: "labeled container stop succeeds when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			ignoreUnlabeled: true,
			err:             nil,
		},
	}
	c := setupDinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t), "stop", tt.ignoreUnlabeled)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDockerClassicProvider_Pause(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) (string, error)
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "non-existing container pause returns provider error",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("No such container: non-existent"),
		},
		{
			name: "labeled container pause succeeds when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			err: nil,
		},
	}
	c := setupDinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t), "pause", true)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
