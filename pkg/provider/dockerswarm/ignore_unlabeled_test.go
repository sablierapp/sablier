package dockerswarm_test

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"gotest.tools/v3/assert"
)

var managedLabels = map[string]string{"sablier.enable": "true"}

func TestDockerSwarmProvider_IgnoreUnlabeled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	p, err := dockerswarm.New(ctx, sharedDinD.client, slogt.New(t), true)
	assert.NilError(t, err)

	t.Run("unlabeled start is rejected", func(t *testing.T) {
		s, err := sharedDinD.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = sharedDinD.client.ServiceRemove(context.Background(), s.ID, client.ServiceRemoveOptions{})
		})
		name := serviceName(t, ctx, s.ID)

		err = p.InstanceStart(ctx, name)
		assert.ErrorContains(t, err, "is not managed by sablier")
	})

	t.Run("unlabeled stop is rejected", func(t *testing.T) {
		s, err := sharedDinD.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = sharedDinD.client.ServiceRemove(context.Background(), s.ID, client.ServiceRemoveOptions{})
		})
		name := serviceName(t, ctx, s.ID)

		err = p.InstanceStop(ctx, name)
		assert.ErrorContains(t, err, "is not managed by sablier")
	})

	t.Run("labeled start and stop succeed", func(t *testing.T) {
		s, err := sharedDinD.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = sharedDinD.client.ServiceRemove(context.Background(), s.ID, client.ServiceRemoveOptions{})
		})
		name := serviceName(t, ctx, s.ID)

		err = p.InstanceStart(ctx, name)
		assert.NilError(t, err)

		err = p.InstanceStop(ctx, name)
		assert.NilError(t, err)
	})
}

func serviceName(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	inspect, err := sharedDinD.client.ServiceInspect(ctx, id, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	return inspect.Service.Spec.Name
}
