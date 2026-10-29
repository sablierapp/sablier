package docker_test

import (
	"context"
	"strings"
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

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
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

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
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

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
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

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
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

// TestDockerClassicProvider_InstanceEvents_Renamed checks the rename events
// against a real daemon, so the "oldName" attribute format is not assumed.
func TestDockerClassicProvider_InstanceEvents_Renamed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{
		sablier.LabelEnable: "true",
		sablier.LabelGroup:  "renamed",
	}})
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	oldName := strings.TrimPrefix(inspected.Container.Name, "/")
	newName := "renamed-" + c.ID[:12]

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventUpdated, provider.InstanceEventRemoved},
	})

	_, err = dind.client.ContainerRename(ctx, c.ID, client.ContainerRenameOptions{NewName: newName})
	assert.NilError(t, err)

	var got []sablier.InstanceEvent
	for len(got) < 2 {
		select {
		case event := <-stream.Events:
			if event.Info.Name == oldName || event.Info.Name == newName {
				got = append(got, event)
			}
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for container rename events: %v", ctx.Err())
		}
	}

	assert.Equal(t, got[0].Type, provider.InstanceEventRemoved)
	assert.Equal(t, got[0].Info.Name, oldName)
	assert.Equal(t, got[1].Type, provider.InstanceEventUpdated)
	assert.Equal(t, got[1].Info.Name, newName)
	assert.DeepEqual(t, got[1].Info.Groups, []string{"renamed"})
}

// subscribe opens an event stream bound to ctx and closes it before the test
// ends, so the provider never logs to a finished test (the race detector
// reports that).
func subscribe(ctx context.Context, t *testing.T, p *docker.Provider, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	stream := p.InstanceEvents(ctx, opts)
	t.Cleanup(func() {
		cancel()
		for range stream.Events {
		}
	})
	return stream
}
