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

func TestDockerClassicProvider_Stop(t *testing.T) {
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
			name: "non existing container stop",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot inspect container: Error response from daemon: No such container: non-existent"),
		},
		{
			name: "container stop as expected",
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
			p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
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
			name: "non existing container pause",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot inspect container: Error response from daemon: No such container: non-existent"),
		},
		{
			name: "container pause as expected",
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

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
