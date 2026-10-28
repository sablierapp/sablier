package dockerswarm_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// managedLabels opts a service in; InstanceEvents ignores the others.
var managedLabels = map[string]string{sablier.LabelEnable: "true"}

func TestDockerSwarmProvider_InstanceEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ServiceRemove(context.Background(), c.ID, client.ServiceRemoveOptions{})
	})

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
			assert.Equal(t, info.Info.Provider, sablier.ProviderSwarm)
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
			assert.Equal(t, info.Info.Provider, sablier.ProviderSwarm)
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

	c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
	assert.NilError(t, err)
	// The service is managed: leaving it behind would show up in the list tests.
	t.Cleanup(func() {
		_, _ = dind.client.ServiceRemove(context.Background(), c.ID, client.ServiceRemoveOptions{})
	})

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
		assert.Equal(t, info.Info.Provider, sablier.ProviderSwarm)
		assert.Assert(t, info.Info.Swarm != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for service started event: %v", ctx.Err())
	}
}

func TestDockerSwarmProvider_InstanceEvents_Created(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventCreated},
	})

	c, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
	assert.NilError(t, err)

	t.Cleanup(func() {
		_, _ = dind.client.ServiceRemove(context.Background(), c.ID, client.ServiceRemoveOptions{})
	})

	inspectResult, err := dind.client.ServiceInspect(ctx, c.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)

	select {
	case info := <-stream.Events:
		assert.Equal(t, info.Info.Name, inspectResult.Service.Spec.Name)
		assert.Equal(t, info.Info.Provider, sablier.ProviderSwarm)
		assert.Assert(t, info.Info.Swarm != nil)
	case err := <-stream.Err:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for service created event: %v", ctx.Err())
	}
}

func TestDockerSwarmProvider_InstanceEvents_UnmanagedIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	unmanaged, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)
	managed, err := dind.CreateMimic(ctx, MimicOptions{Labels: managedLabels})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ServiceRemove(context.Background(), unmanaged.ID, client.ServiceRemoveOptions{})
		_, _ = dind.client.ServiceRemove(context.Background(), managed.ID, client.ServiceRemoveOptions{})
	})
	unmanagedName := serviceName(ctx, t, dind.client, unmanaged.ID)
	managedName := serviceName(ctx, t, dind.client, managed.ID)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{})

	scale := func(id string, replicas uint64) {
		t.Helper()
		inspectResult, err := dind.client.ServiceInspect(ctx, id, client.ServiceInspectOptions{})
		assert.NilError(t, err)
		service := inspectResult.Service
		service.Spec.Mode.Replicated.Replicas = &replicas
		_, err = p.Client.ServiceUpdate(ctx, service.ID, client.ServiceUpdateOptions{Version: service.Version, Spec: service.Spec})
		assert.NilError(t, err)
	}
	remove := func(id string) {
		t.Helper()
		_, err := p.Client.ServiceRemove(ctx, id, client.ServiceRemoveOptions{})
		assert.NilError(t, err)
	}
	// expect waits for the managed service's event. The unmanaged service
	// changes first, so if it were reported, its event would arrive before.
	expect := func(want provider.InstanceEventType) {
		t.Helper()
		for {
			select {
			case ev := <-stream.Events:
				assert.Assert(t, ev.Info.Name != unmanagedName, "unmanaged service reported: %+v", ev)
				if ev.Info.Name == managedName && ev.Type == want {
					return
				}
			case err := <-stream.Err:
				t.Fatalf("unexpected error: %v", err)
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %s event: %v", want, ctx.Err())
			}
		}
	}

	scale(unmanaged.ID, 0)
	scale(managed.ID, 0)
	expect(provider.InstanceEventStopped)

	scale(unmanaged.ID, 1)
	scale(managed.ID, 1)
	expect(provider.InstanceEventStarted)

	remove(unmanaged.ID)
	remove(managed.ID)
	expect(provider.InstanceEventRemoved)
}

func serviceName(ctx context.Context, t *testing.T, cli *client.Client, id string) string {
	t.Helper()
	inspectResult, err := cli.ServiceInspect(ctx, id, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	return inspectResult.Service.Spec.Name
}
