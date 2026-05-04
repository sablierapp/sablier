package podman_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
)

var managedLabels = map[string]string{"sablier.enable": "true"}

func TestPodmanProvider_Start(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(dind *pindContainer) (string, error)
	}
	tests := []struct {
		name            string
		args            args
		ignoreUnlabeled bool
		err             error
	}{
		{
			name: "non existing container start",
			args: args{
				do: func(dind *pindContainer) (string, error) {
					return "non-existent", nil
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("no such container"),
		},
		{
			name: "unlabeled container start",
			args: args{
				do: func(dind *pindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					return c.ID, err
				},
			},
			ignoreUnlabeled: true,
			err:             fmt.Errorf("is not managed by sablier"),
		},
		{
			name: "unlabeled container start when allowed",
			args: args{
				do: func(dind *pindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					return c.ID, err
				},
			},
			ignoreUnlabeled: false,
			err:             nil,
		},
		{
			name: "container start as expected",
			args: args{
				do: func(dind *pindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
					return c.ID, err
				},
			},
			ignoreUnlabeled: true,
			err:             nil,
		},
	}
	c := setupPinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := podman.New(c.connText, slogt.New(t), tt.ignoreUnlabeled)
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
