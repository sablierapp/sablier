package podman_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
)

func TestPodmanProvider_Stop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(pind *pindContainer) (string, error)
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "non existing container stop",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			err: fmt.Errorf("cannot stop container non-existent: no container with name or ID \"non-existent\" found: no such container"),
		},
		{
			name: "container stop as expected",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					err = containers.Start(pind.connText, c.ID, nil)
					if err != nil {
						return "", err
					}

					return c.ID, nil
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

			err = p.InstanceStop(t.Context(), name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
