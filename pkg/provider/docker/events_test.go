package docker_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t)
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
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

func TestDockerProvider_EventsHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t)
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	msgs, _ := p.Events(ctx)

	opts := MimicOptions{
		Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy", "-healthy-after=1ms"},
		Healthcheck: &container.HealthConfig{
			Test:          []string{"CMD", "/mimic", "healthcheck"},
			Interval:      100 * time.Millisecond,
			Timeout:       time.Second,
			StartPeriod:   time.Second,
			StartInterval: 100 * time.Millisecond,
			Retries:       10,
		},
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "test",
		},
	}

	c, err := dind.CreateMimic(ctx, opts)
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID)
	assert.NilError(t, err)

	name := strings.TrimPrefix(inspected.Name, "/") // Docker container name is prefixed with a slash, but we don't use it

	// We should receive the create event
	evt := receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionCreated)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
	assert.NilError(t, err)

	// We should receive the start event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionPending)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	// We should receive the healthy event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRunning)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerStop(ctx, c.ID, container.StopOptions{})
	assert.NilError(t, err)

	// We should receive the stopping event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopping)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	// We should receive the stopped event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopped)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{})
	assert.NilError(t, err)

	// We should receive the remove event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRemoved)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")
}

func TestDockerProvider_EventsNoHealthcheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t)
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	msgs, _ := p.Events(ctx)

	opts := MimicOptions{
		Cmd: []string{"/mimic", "-running", "-running-after=1ms"},
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "test",
		},
	}

	c, err := dind.CreateMimic(ctx, opts)
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID)
	assert.NilError(t, err)

	name := strings.TrimPrefix(inspected.Name, "/") // Docker container name is prefixed with a slash, but we don't use it

	// We should receive the create event
	evt := receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionCreated)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
	assert.NilError(t, err)

	// We should receive the start event as a Running event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRunning)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerStop(ctx, c.ID, container.StopOptions{})
	assert.NilError(t, err)

	// We should receive the stopping event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopping)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	// We should receive the stopped event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopped)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{})
	assert.NilError(t, err)

	// We should receive the remove event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRemoved)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")
}

func TestDockerProvider_EventsNoHealthcheckPause(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t)
	p, err := docker.New(ctx, dind.client, slogt.New(t), "pause")
	assert.NilError(t, err)

	msgs, _ := p.Events(ctx)

	opts := MimicOptions{
		Cmd: []string{"/mimic", "-running", "-running-after=1ms"},
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "test",
		},
	}

	c, err := dind.CreateMimic(ctx, opts)
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID)
	assert.NilError(t, err)

	name := strings.TrimPrefix(inspected.Name, "/") // Docker container name is prefixed with a slash, but we don't use it

	// We should receive the create event
	evt := receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionCreated)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
	assert.NilError(t, err)

	// We should receive the start event as a Running event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRunning)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerPause(ctx, c.ID)
	assert.NilError(t, err)

	// We should receive the stopped event immediatly
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopped)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerUnpause(ctx, c.ID)
	assert.NilError(t, err)

	// We should receive the stopped event immediatly
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRunning)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerStop(ctx, c.ID, container.StopOptions{})
	assert.NilError(t, err)

	// We should receive the stopping event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopping)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	// We should receive the stopped event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionStopped)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")

	err = dind.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{})
	assert.NilError(t, err)

	// We should receive the remove event
	evt = receiveWithTimeout(t, ctx, msgs)
	assert.Equal(t, evt.Type, provider.EventTypeInstance)
	assert.Equal(t, evt.Action, provider.EventActionRemoved)
	assert.Equal(t, evt.InstanceName, name)
	assert.Equal(t, evt.ProviderName, "docker")
}

func receiveWithTimeout(t *testing.T, ctx context.Context, ch <-chan provider.Event) provider.Event {
	t.Helper()
	select {
	case evt := <-ch:
		return evt
	case <-ctx.Done():
		t.Fatal("timeout while waiting for event")
		return provider.Event{}
	}
}
