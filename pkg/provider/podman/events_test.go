package podman_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestPodmanProvider_InstanceEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pind := sharedPinD
	p, err := podman.New(ctx, pind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := pind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := pind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	err = WaitForContainerRunning(ctx, pind.client, c.ID)
	assert.NilError(t, err)

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	_, err = pind.client.ContainerStop(ctx, c.ID, client.ContainerStopOptions{})
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		// Podman may or may not prefix container names with "/" — compare without the slash.
		assert.Equal(t, info.Info.Name, strings.TrimPrefix(inspected.Container.Name, "/"))
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodmanProvider_InstanceEvents_Started(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pind := sharedPinD
	p, err := podman.New(ctx, pind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := pind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := pind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	stream := subscribe(ctx, t, p, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	err = WaitForContainerRunning(ctx, pind.client, c.ID)
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		assert.Equal(t, info.Info.Name, strings.TrimPrefix(inspected.Container.Name, "/"))
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	}
}

// subscribe opens an event stream bound to ctx and closes it before the test
// ends, so the provider never logs to a finished test (the race detector
// reports that).
func subscribe(ctx context.Context, t *testing.T, p *podman.Provider, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
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
