package docker_test

import (
	"context"
	"fmt"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
	"testing"
)

func TestDockerClassicProvider_Start(t *testing.T) {
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
			name: "non existing container start",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot start container non-existent: Error response from daemon: No such container: non-existent"),
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
	c := setupDinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t))
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
