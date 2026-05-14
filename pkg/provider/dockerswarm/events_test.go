package dockerswarm_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"gotest.tools/v3/assert"
)

func TestDockerSwarmProvider_InstanceEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	t.Run("service is scaled to 0 replicas", func(t *testing.T) {
		inspectResult, err := dind.client.ServiceInspect(ctx, c.ID, client.ServiceInspectOptions{})
		assert.NilError(t, err)
		service := inspectResult.Service

		replicas := uint64(0)
		service.Spec.Mode.Replicated.Replicas = &replicas

		_, err = p.Client.ServiceUpdate(ctx, service.ID, client.ServiceUpdateOptions{Version: service.Version, Spec: service.Spec})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Info.Name, service.Spec.Name)
			assert.Equal(t, info.Info.Provider, "swarm")
			assert.Assert(t, info.Info.Swarm != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("service is removed", func(t *testing.T) {
		inspectResult, err := dind.client.ServiceInspect(ctx, c.ID, client.ServiceInspectOptions{})
		assert.NilError(t, err)
		service := inspectResult.Service

		_, err = p.Client.ServiceRemove(ctx, service.ID, client.ServiceRemoveOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Info.Name, service.Spec.Name) // Service is removed; provider is still set.
			assert.Equal(t, info.Info.Provider, "swarm")
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDockerSwarmProvider_InstanceEvents_Started(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspectResult, err := dind.client.ServiceInspect(ctx, c.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	service := inspectResult.Service

	replicas := uint64(0)
	service.Spec.Mode.Replicated.Replicas = &replicas
	_, err = p.Client.ServiceUpdate(ctx, service.ID, client.ServiceUpdateOptions{Version: service.Version, Spec: service.Spec})
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	inspectResult, err = dind.client.ServiceInspect(ctx, c.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	service = inspectResult.Service
	replicas = uint64(1)
	service.Spec.Mode.Replicated.Replicas = &replicas
	_, err = p.Client.ServiceUpdate(ctx, service.ID, client.ServiceUpdateOptions{Version: service.Version, Spec: service.Spec})
	assert.NilError(t, err)

	err = WaitForServiceRunning(ctx, dind.client, service.Spec.Name, 1)
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		assert.Equal(t, info.Info.Name, service.Spec.Name)
		assert.Equal(t, info.Info.Provider, "swarm")
		assert.Assert(t, info.Info.Swarm != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for service started event: %v", ctx.Err())
	}
}
