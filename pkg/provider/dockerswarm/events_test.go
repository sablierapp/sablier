package dockerswarm_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/api/types/swarm"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"gotest.tools/v3/assert"
)

func TestDockerSwarmProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t)
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	waitC := make(chan string)
	go p.NotifyInstanceStopped(ctx, waitC)

	t.Run("service is scaled to 0 replicas", func(t *testing.T) {
		service, _, err := dind.client.ServiceInspectWithRaw(ctx, c.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		replicas := uint64(0)
		service.Spec.Mode.Replicated.Replicas = &replicas

		_, err = p.Client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, swarm.ServiceUpdateOptions{})
		assert.NilError(t, err)

		name := <-waitC

		// Docker container name is prefixed with a slash, but we don't use it
		assert.Equal(t, name, service.Spec.Name)
	})

	t.Run("service is removed", func(t *testing.T) {
		service, _, err := dind.client.ServiceInspectWithRaw(ctx, c.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		err = p.Client.ServiceRemove(ctx, service.ID)
		assert.NilError(t, err)

		name := <-waitC

		// Docker container name is prefixed with a slash, but we don't use it
		assert.Equal(t, name, service.Spec.Name)
	})
}
