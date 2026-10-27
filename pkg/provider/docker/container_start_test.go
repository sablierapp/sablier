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
		name string
		args args
		err  error
	}{
		{
			name: "non existing container start",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot inspect container: Error response from daemon: No such container: non-existent"),
		},
		{
			name: "container start as expected",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					return c.ID, err
				},
			},
			err: nil,
		},
	}
	c := sharedDinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStart(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
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
			name: "non existing container unpause",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot inspect container: Error response from daemon: No such container: non-existent"),
		},
		{
			name: "container starts because was not paused",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
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
			name: "container unpause as expected",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
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
			p, err := docker.New(ctx, c.client, slogt.New(t), "pause")
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStart(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

// A container paused outside Sablier must start, whatever the strategy:
// docker start refuses a paused container. See #1181.
func TestDockerClassicProvider_Start_PausedContainerWithStopStrategy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD

	result, err := c.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)
	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)
	_, err = c.client.ContainerPause(ctx, result.ID, client.ContainerPauseOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), result.ID)
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, spec.Container.State.Running, true)
	assert.Equal(t, spec.Container.State.Paused, false)
}
