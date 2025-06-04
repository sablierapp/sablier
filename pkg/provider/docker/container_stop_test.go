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
	doFactory := func(addLabel bool, labelValue string) func(dind *dindContainer) (string, error) {
		return func(dind *dindContainer) (string, error) {
			opts := MimicOptions{}
			if addLabel {
				opts = MimicOptions{Labels: map[string]string{"sablier.pauseOnly": labelValue}}
			}
			c, err := dind.CreateMimic(ctx, opts)
			if err != nil {
				return "", err
			}

			err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
			if err != nil {
				return "", err
			}

			return c.ID, nil
		}
	}
	type args struct {
		do func(dind *dindContainer) (string, error)
	}
	tests := []struct {
		name       string
		args       args
		err        error
		assertions func(dind *dindContainer, id string)
	}{
		{
			name: "non existing container stop",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err:        fmt.Errorf("cannot stop container non-existent: Error response from daemon: No such container: non-existent"),
			assertions: func(dind *dindContainer, id string) {},
		},
		{
			name: "container stops as expected without pauseOnly label",
			args: args{
				do: doFactory(false, ""),
			},
			err: nil,
			assertions: func(dind *dindContainer, id string) {
				inspectResponse, _ := dind.client.ContainerInspect(ctx, id)
				assert.Equal(t, inspectResponse.State.Status, "exited")
			},
		},
		{
			name: "container stops as expected with pauseOnly label set to false",
			args: args{
				do: doFactory(true, "false"),
			},
			err: nil,
			assertions: func(dind *dindContainer, id string) {
				inspectResponse, _ := dind.client.ContainerInspect(ctx, id)
				assert.Equal(t, inspectResponse.State.Status, "exited")
			},
		},
		{
			name: "container pauses as expected",
			args: args{
				do: doFactory(true, "true"),
			},
			err: nil,
			assertions: func(dind *dindContainer, id string) {
				inspectResponse, err := dind.client.ContainerInspect(ctx, id)
				assert.NilError(t, err)
				assert.Equal(t, inspectResponse.State.Status, "paused")
			},
		},
	}
	c := setupDinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t))
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}

			tt.assertions(c, name)
		})
	}
}
