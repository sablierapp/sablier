package docker_test

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

// TestDockerScaleMode_Stop verifies that InstanceStop applies idle resource limits
// instead of stopping the container when scale labels are set.
func TestDockerScaleMode_Stop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD

	// Create a container with idle/active scale labels
	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.idle.cpu":    "0.5",
			"sablier.idle.memory": "128m",
			"sablier.active.cpu":  "2.0",
			"sablier.active.memory": "512m",
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	// InstanceStop should apply idle resources, not stop the container
	err = p.InstanceStop(t.Context(), result.ID)
	assert.NilError(t, err)

	// Container should still be running
	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running",
		"container should still be running in scale mode")

	// CPU limit should be set to idle value (0.5 cores = 500_000_000 nanocores)
	assert.Equal(t, spec.Container.HostConfig.NanoCPUs, int64(500_000_000),
		"CPU should be throttled to idle value")

	// Memory limit should be set to idle value (128 MiB = 134_217_728 bytes)
	assert.Equal(t, spec.Container.HostConfig.Memory, int64(128*1024*1024),
		"Memory should be throttled to idle value")
}

// TestDockerScaleMode_Start verifies that InstanceStart applies active resource limits
// instead of starting the container when scale labels are set.
func TestDockerScaleMode_Start(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD

	// Create a container with scale labels and idle resources already applied
	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.idle.cpu":      "0.5",
			"sablier.idle.memory":   "128m",
			"sablier.active.cpu":    "2.0",
			"sablier.active.memory": "512m",
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	// InstanceStart should apply active resources
	err = p.InstanceStart(t.Context(), result.ID)
	assert.NilError(t, err)

	// Container should still be running
	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running")

	// CPU limit should be set to active value (2.0 cores = 2_000_000_000 nanocores)
	assert.Equal(t, spec.Container.HostConfig.NanoCPUs, int64(2_000_000_000),
		"CPU should be set to active value")

	// Memory limit should be set to active value (512 MiB = 536_870_912 bytes)
	assert.Equal(t, spec.Container.HostConfig.Memory, int64(512*1024*1024),
		"Memory should be set to active value")
}
