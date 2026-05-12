package docker_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

var managedLabels = map[string]string{"sablier.enable": "true"}

func TestDockerClassicProvider_Start(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

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
			name: "non-existing container start returns provider error",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("No such container: non-existent"),
		},
		{
			name: "unlabeled container start is rejected when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					return c.ID, err
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("is not managed by sablier"),
		},
		{
			name: "unlabeled container start succeeds when ignoreUnlabeled is disabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					return c.ID, err
				},
			},
			ignoreUnlabeled: false,
			err:             nil,
		},
		{
			name: "labeled container start succeeds when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
					return c.ID, err
				},
			},
			ignoreUnlabeled: true,
			err:             nil,
		},
	}
	c := sharedDinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t), "stop", tt.ignoreUnlabeled)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStart(t.Context(), name)
			if tt.err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDockerClassicProvider_Unpause(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

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
			name: "non-existing container unpause returns provider error",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("No such container: non-existent"),
		},
		{
			name: "labeled stopped container starts when pause strategy cannot unpause",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStop(ctx, c.ID, client.ContainerStopOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			err: nil,
		},
		{
			name: "labeled paused container unpauses when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerPause(ctx, c.ID, client.ContainerPauseOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			err: nil,
		},
	}
	c := sharedDinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t), "pause", true)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStart(t.Context(), name)
			if tt.err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
