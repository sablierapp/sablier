package podman_test

// Scale mode against a real Podman. Sablier updates resource limits through
// Podman's Docker-compatible API; these tests check which limits Podman
// actually applies. See #1189.

import (
	"context"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
)

// idleScaleContainer creates and starts a container with the given idle
// labels in scale mode, and has Sablier apply the idle profile.
func idleScaleContainer(t *testing.T, labels map[string]string) client.ContainerInspectResult {
	t.Helper()
	ctx := context.Background()
	c := sharedPinD

	labels["sablier.enable"] = "true"
	labels["sablier.idle.replicas"] = "1"
	result, err := c.CreateMimic(ctx, MimicOptions{Labels: labels})
	assert.NilError(t, err)
	_, err = c.client.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	p, err := podman.New(ctx, c.client, slogt.New(t))
	assert.NilError(t, err)
	err = p.InstanceStop(t.Context(), result.ID)
	if err != nil && strings.Contains(err.Error(), "crun update") {
		// Podman accepted the update but its runtime could not apply it: the
		// nested Podman used in CI has no delegated cgroup controllers.
		t.Skipf("the container runtime cannot update cgroup resources in this environment: %v", err)
	}
	assert.NilError(t, err)

	spec, err := c.client.ContainerInspect(ctx, result.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(spec.Container.State.Status), "running", "scale mode must keep the container running")
	return spec
}

func TestPodmanScaleMode_IdleCPU(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	spec := idleScaleContainer(t, map[string]string{"sablier.idle.cpu": "0.5"})

	// Podman may report the limit as NanoCPUs or as a CFS quota and period.
	hc := spec.Container.HostConfig
	cores := float64(hc.NanoCPUs) / 1e9
	if hc.NanoCPUs == 0 && hc.CPUPeriod > 0 {
		cores = float64(hc.CPUQuota) / float64(hc.CPUPeriod)
	}
	assert.Equal(t, cores, 0.5, "idle CPU limit (NanoCPUs=%d CPUQuota=%d CPUPeriod=%d)", hc.NanoCPUs, hc.CPUQuota, hc.CPUPeriod)
}

func TestPodmanScaleMode_IdleMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	spec := idleScaleContainer(t, map[string]string{"sablier.idle.memory": "128m"})
	assert.Equal(t, spec.Container.HostConfig.Memory, int64(128*1024*1024), "idle memory limit")
}

func TestPodmanScaleMode_IdleBlkioWeight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	spec := idleScaleContainer(t, map[string]string{"sablier.idle.blkio-weight": "100"})
	assert.Equal(t, spec.Container.HostConfig.BlkioWeight, uint16(100), "idle block I/O weight")
}
