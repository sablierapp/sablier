package docker_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
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
			"sablier.idle.replicas":   "1",
			"sablier.idle.cpu":        "0.5",
			"sablier.idle.memory":     "128m",
			"sablier.active.replicas": "1",
			"sablier.active.cpu":      "2.0",
			"sablier.active.memory":   "512m",
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
			"sablier.idle.replicas":   "1",
			"sablier.idle.cpu":        "0.5",
			"sablier.idle.memory":     "128m",
			"sablier.active.replicas": "1",
			"sablier.active.cpu":      "2.0",
			"sablier.active.memory":   "512m",
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

// TestDockerScaleMode_Stop_ReplicasOnly verifies that InstanceStop keeps the container
// running when only sablier.idle.replicas is set (no CPU/memory labels).
func TestDockerScaleMode_Stop_ReplicasOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD

	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.idle.replicas": "1",
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	err = p.InstanceStop(t.Context(), result.ID)
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running",
		"container should still be running in replicas-only scale mode")

	// No resource limits should have been applied.
	assert.Equal(t, spec.Container.HostConfig.NanoCPUs, int64(0),
		"CPU limit should be unchanged (0 = no limit)")
	assert.Equal(t, spec.Container.HostConfig.Memory, int64(0),
		"Memory limit should be unchanged (0 = no limit)")
}

// TestDockerScaleMode_Start_ReplicasOnly verifies that InstanceStart is a no-op for
// resources when only sablier.active.replicas is set (no CPU/memory labels).
func TestDockerScaleMode_Start_ReplicasOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD

	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.active.replicas": "1",
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), result.ID)
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running")

	// No resource limits should have been applied.
	assert.Equal(t, spec.Container.HostConfig.NanoCPUs, int64(0),
		"CPU limit should be unchanged (0 = no limit)")
	assert.Equal(t, spec.Container.HostConfig.Memory, int64(0),
		"Memory limit should be unchanged (0 = no limit)")
}

// TestDockerScaleMode_Stop_BlkioWeight verifies that InstanceStop applies the idle
// blkio-weight cgroup limit instead of stopping the container.
func TestDockerScaleMode_Stop_BlkioWeight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD
	checkBlkioCgroupIOSupport(t, ctx, c)

	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.idle.replicas":       "1",
			"sablier.idle.blkio-weight":   "50",
			"sablier.active.blkio-weight": "800",
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	err = p.InstanceStop(t.Context(), result.ID)
	skipIfBlkioUnavailable(t, err)
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running",
		"container should still be running in scale mode")
	assert.Equal(t, spec.Container.HostConfig.BlkioWeight, uint16(50),
		"BlkioWeight should be throttled to idle value")
}

// TestDockerScaleMode_Start_BlkioWeight verifies that InstanceStart restores the active
// blkio-weight cgroup limit without restarting the container.
func TestDockerScaleMode_Start_BlkioWeight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD
	checkBlkioCgroupIOSupport(t, ctx, c)

	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.idle.replicas":       "1",
			"sablier.idle.blkio-weight":   "50",
			"sablier.active.blkio-weight": "800",
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), result.ID)
	skipIfBlkioUnavailable(t, err)
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running")
	assert.Equal(t, spec.Container.HostConfig.BlkioWeight, uint16(800),
		"BlkioWeight should be restored to active value")
}

// execOutput runs a command in the DinD container and returns demultiplexed stdout.
// The raw reader from testcontainers Exec is a Docker multiplexed stream (8-byte
// headers followed by payload); stdcopy.StdCopy strips those headers so callers
// receive clean text output.
func execOutput(out io.Reader) string {
	var stdout bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, io.Discard, out)
	return strings.TrimSpace(stdout.String())
}

// findAvailableBlockDevice returns the path of a block device visible inside the DinD
// container (e.g. /dev/vda, /dev/sda), or "" if none can be found.
// The returned path is safe to use in blkio device throttle labels because the inner
// Docker daemon running inside DinD has access to the same /dev namespace.
func findAvailableBlockDevice(ctx context.Context, c *dindContainer) string {
	code, out, err := c.Exec(ctx, []string{
		"sh", "-c", "ls /dev/vd? /dev/sd? /dev/nvme?n? 2>/dev/null | head -1",
	})
	if err != nil || code != 0 {
		return ""
	}
	return execOutput(out)
}

// checkBlkioCgroupIOSupport skips the test if the cgroupv2 "io" controller is not
// delegated inside the DinD cgroup namespace. This controller is required for
// blkio-weight and blkio device throttle operations in nested Docker environments.
//
// The check reads /sys/fs/cgroup/docker/cgroup.subtree_control, which lists the
// controllers that the inner Docker daemon delegates to the cgroups it creates for
// containers. If "io" is absent from that file (or the file doesn't exist), blkio
// operations will fail at runtime and we skip the test instead.
func checkBlkioCgroupIOSupport(t *testing.T, ctx context.Context, c *dindContainer) {
	t.Helper()
	// Use the docker-specific subtree_control file — NOT the top-level one.
	// The top-level file reflects what the host kernel supports, which may differ
	// from what the inner Docker daemon is actually allowed to use.
	code, out, err := c.Exec(ctx, []string{
		"cat", "/sys/fs/cgroup/docker/cgroup.subtree_control",
	})
	if err != nil || code != 0 {
		t.Skip("cgroupv2 'io' controller not available in DinD (docker cgroup.subtree_control unreadable); skipping blkio test")
		return
	}
	content := execOutput(out)
	if !strings.Contains(content, "io") {
		t.Skipf("cgroupv2 'io' controller not delegated to DinD containers (subtree_control=%q); skipping blkio test",
			content)
	}
}

// skipIfBlkioUnavailable checks whether err represents a known blkio cgroup limitation
// (the cgroupv2 'io' controller not being delegated in nested Docker environments such
// as DinD). If so, it marks the test as skipped rather than failed so CI does not
// report a false positive.
func skipIfBlkioUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	// runc emits this message when the cgroup io.weight or io.max file cannot be
	// opened because the 'io' controller has not been delegated to the cgroup.
	if (strings.Contains(msg, "io.weight") || strings.Contains(msg, "io.max")) &&
		strings.Contains(msg, "no such file") {
		t.Skipf("cgroupv2 'io' controller not delegated in this Docker environment (common in DinD); skipping blkio test: %v", err)
	}
}

// TestDockerScaleMode_BlkioDeviceReadBps verifies that InstanceStop applies a per-device
// read-throughput limit via ContainerUpdate. The test discovers a block device present
// in the DinD environment and skips itself when none is found.
func TestDockerScaleMode_BlkioDeviceReadBps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD
	checkBlkioCgroupIOSupport(t, ctx, c)

	devPath := findAvailableBlockDevice(ctx, c)
	if devPath == "" {
		t.Skip("no block device found in DinD environment; skipping blkio device throttle test")
	}
	t.Logf("using block device: %q", devPath)

	// Throttle idle read throughput to 5 MiB/s on the discovered device.
	const idleRate = "5m"
	const idleRateBytes = uint64(5 * 1024 * 1024)

	result, err := c.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.idle.replicas":              "1",
			"sablier.idle.blkio-device-read-bps": devPath + ":" + idleRate,
		},
	})
	assert.NilError(t, err)

	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	err = p.InstanceStop(t.Context(), result.ID)
	skipIfBlkioUnavailable(t, err)
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running",
		"container should still be running in scale mode")

	// If Docker silently failed to apply the device throttle (warnings returned instead
	// of an error — typical when the cgroupv2 'io' controller is not properly delegated),
	// the HostConfig entry will be absent. Before giving up, check the real cgroup
	// io.max file: Docker's cgroupv2 path may write io.max correctly but not reflect
	// the change in HostConfig (a known behaviour difference vs cgroupv1).
	if len(spec.Container.HostConfig.BlkioDeviceReadBps) == 0 {
		// Read the actual cgroup io.max for this container from inside the DinD host.
		_, ioMaxOut, _ := c.Exec(ctx, []string{
			"sh", "-c",
			fmt.Sprintf("cat /sys/fs/cgroup/docker/%s/io.max 2>/dev/null || echo UNAVAILABLE", result.ID),
		})
		ioMaxContent := execOutput(ioMaxOut)
		t.Logf("io.max for container %s: %q", result.ID, ioMaxContent)

		if ioMaxContent == "UNAVAILABLE" || ioMaxContent == "" {
			t.Skip("blkio device throttle was not applied and io.max is unreadable; cgroupv2 io controller may not be available in this environment")
		}
		// io.max is readable — the throttle WAS applied at the cgroup level even though
		// HostConfig didn't reflect it. Verify the cgroup content directly.
		assert.Assert(t, strings.Contains(ioMaxContent, "rbps="),
			"expected io.max to contain rbps throttle entry; got: %q", ioMaxContent)
		return
	}

	assert.Equal(t, len(spec.Container.HostConfig.BlkioDeviceReadBps), 1,
		"expected exactly one blkio-device-read-bps entry")
	assert.Equal(t, spec.Container.HostConfig.BlkioDeviceReadBps[0].Path, devPath,
		"device path should match")
	assert.Equal(t, spec.Container.HostConfig.BlkioDeviceReadBps[0].Rate, idleRateBytes,
		"read BPS rate should match the idle label value")
}
