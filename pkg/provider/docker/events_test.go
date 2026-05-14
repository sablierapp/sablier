package docker_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_InstanceEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	err = WaitForContainerRunning(ctx, dind.client, c.ID)
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	_, err = dind.client.ContainerStop(ctx, c.ID, client.ContainerStopOptions{})
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		// Docker container name is prefixed with a slash, but we don't use it
		assert.Equal(t, "/"+info.Info.Name, inspected.Container.Name)
		assert.Equal(t, info.Info.Provider, "docker")
		assert.Assert(t, info.Info.Docker != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerClassicProvider_InstanceEvents_Started(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	err = WaitForContainerRunning(ctx, dind.client, c.ID)
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		assert.Equal(t, "/"+info.Info.Name, inspected.Container.Name)
		assert.Equal(t, info.Info.Provider, "docker")
		assert.Assert(t, info.Info.Docker != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	}
}
