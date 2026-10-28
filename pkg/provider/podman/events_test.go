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

	c, err := pind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{sablier.LabelEnable: "true"}})
	assert.NilError(t, err)

	inspected, err := pind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	err = WaitForContainerRunning(ctx, pind.client, c.ID)
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
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

	c, err := pind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{sablier.LabelEnable: "true"}})
	assert.NilError(t, err)

	inspected, err := pind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
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

func TestPodmanProvider_InstanceEvents_IgnoresUnmanaged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pind := sharedPinD
	p, err := podman.New(ctx, pind.client, slogt.New(t))
	assert.NilError(t, err)

	unmanaged, err := pind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)
	managed, err := pind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{sablier.LabelEnable: "true"}})
	assert.NilError(t, err)
	inspected, err := pind.client.ContainerInspect(ctx, managed.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	// Start the unmanaged container first: the managed container's event can
	// only be the first one received if the unmanaged one was filtered out.
	_, err = pind.client.ContainerStart(ctx, unmanaged.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)
	err = WaitForContainerRunning(ctx, pind.client, unmanaged.ID)
	assert.NilError(t, err)
	_, err = pind.client.ContainerStart(ctx, managed.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		assert.Equal(t, info.Info.Name, strings.TrimPrefix(inspected.Container.Name, "/"))
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for managed container started event: %v", ctx.Err())
	}
}
