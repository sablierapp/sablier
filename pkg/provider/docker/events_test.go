package docker_test

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestDockerClassicProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t, ctx)
	p, err := docker.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID)
	assert.NilError(t, err)

	err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
	assert.NilError(t, err)

	<-time.After(1 * time.Second)

	waitC := make(chan string)
	go p.NotifyInstanceStopped(ctx, waitC)

	err = dind.client.ContainerStop(ctx, c.ID, container.StopOptions{})
	assert.NilError(t, err)

	name := <-waitC

	// Docker container name is prefixed with a slash, but we don't use it
	assert.Equal(t, "/"+name, inspected.Name)
}
