package podman_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
)

func TestPodmanProvider_Start(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(dind *pindContainer) (string, error)
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "non existing container start",
			args: args{
				do: func(dind *pindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot start container non-existent: no container with name or ID \"non-existent\" found: no such container"),
		},
		{
			name: "container start as expected",
			args: args{
				do: func(dind *pindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					return c.ID, err
				},
			},
			err: nil,
		},
	}
	c := setupPinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := podman.New(c.connText, slogt.New(t))
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
