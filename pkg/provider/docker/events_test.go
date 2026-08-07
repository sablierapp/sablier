package docker_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
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
		assert.Equal(t, info.Info.Provider, sablier.ProviderDocker)
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
		assert.Equal(t, info.Info.Provider, sablier.ProviderDocker)
		assert.Assert(t, info.Info.Docker != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerClassicProvider_InstanceEvents_Created(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated},
	})

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		assert.Equal(t, "/"+info.Info.Name, inspected.Container.Name)
		assert.Equal(t, info.Info.Provider, sablier.ProviderDocker)
		assert.Assert(t, info.Info.Docker != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for container created event: %v", ctx.Err())
	}
}

func TestDockerClassicProvider_InstanceEvents_Removed(t *testing.T) {
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
		Types: []provider.InstanceEventType{provider.InstanceEventRemoved},
	})

	_, err = dind.client.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{})
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		// Container is gone; only the name is available, no Docker-specific info.
		assert.Equal(t, "/"+info.Info.Name, inspected.Container.Name)
		assert.Equal(t, info.Info.Provider, sablier.ProviderDocker)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for container removed event: %v", ctx.Err())
	}
}
