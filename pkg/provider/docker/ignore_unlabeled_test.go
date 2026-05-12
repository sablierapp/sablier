package docker_test

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

var managedLabels = map[string]string{"sablier.enable": "true"}

func TestDockerClassicProvider_IgnoreUnlabeled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	p, err := docker.New(ctx, sharedDinD.client, slogt.New(t), "stop", true)
	assert.NilError(t, err)

	t.Run("unlabeled start is rejected", func(t *testing.T) {
		c, err := sharedDinD.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = sharedDinD.client.ContainerRemove(context.Background(), c.ID, client.ContainerRemoveOptions{Force: true})
		})

		err = p.InstanceStart(ctx, c.ID)
		assert.ErrorContains(t, err, "is not managed by sablier")
	})

	t.Run("unlabeled stop is rejected", func(t *testing.T) {
		c, err := sharedDinD.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = sharedDinD.client.ContainerRemove(context.Background(), c.ID, client.ContainerRemoveOptions{Force: true})
		})

		_, err = sharedDinD.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
		assert.NilError(t, err)

		err = p.InstanceStop(ctx, c.ID)
		assert.ErrorContains(t, err, "is not managed by sablier")
	})

	t.Run("labeled start and stop succeed", func(t *testing.T) {
		c, err := sharedDinD.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = sharedDinD.client.ContainerRemove(context.Background(), c.ID, client.ContainerRemoveOptions{Force: true})
		})

		err = p.InstanceStart(ctx, c.ID)
		assert.NilError(t, err)

		err = p.InstanceStop(ctx, c.ID)
		assert.NilError(t, err)
	})
}
