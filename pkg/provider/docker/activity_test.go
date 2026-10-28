package docker_test

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDockerProvider_InstanceActivityTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{
		ExposedPorts: network.PortSet{
			network.MustParsePort("8080/tcp"): {},
			network.MustParsePort("5353/udp"): {},
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ContainerRemove(t.Context(), c.ID, client.ContainerRemoveOptions{Force: true})
	})

	info, err := dind.client.Info(ctx, client.InfoOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceActivityTarget(ctx, c.ID)
	assert.NilError(t, err)
	t.Logf("cgroup driver=%q path=%q", info.Info.CgroupDriver, got.CgroupPath)

	assert.Assert(t, strings.Contains(got.CgroupPath, c.ID), got.CgroupPath)
	t.Logf("ports=%v", got.Ports) // also includes the ports the image itself EXPOSEs
	assert.Assert(t, slices.Contains(got.Ports, sablier.Port{Number: 8080, Proto: "tcp"}))
	assert.Assert(t, slices.Contains(got.Ports, sablier.Port{Number: 5353, Proto: "udp"}))
}

func TestDockerProvider_ActivityIgnoredSources(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	start := func(labels map[string]string) string {
		c, err := dind.CreateMimic(ctx, MimicOptions{Labels: labels})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_, _ = dind.client.ContainerRemove(t.Context(), c.ID, client.ContainerRemoveOptions{Force: true})
		})
		_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
		assert.NilError(t, err)
		assert.NilError(t, WaitForContainerRunning(ctx, dind.client, c.ID))
		return c.ID
	}

	ignored := start(map[string]string{sablier.LabelActivityIgnoreSource: "true"})
	_ = start(nil)

	insp, err := dind.client.ContainerInspect(ctx, ignored, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	var want []netip.Addr
	for _, ep := range insp.Container.NetworkSettings.Networks {
		if ep.IPAddress.IsValid() {
			want = append(want, ep.IPAddress)
		}
	}
	assert.Assert(t, len(want) > 0)

	got, err := p.ActivityIgnoredSources(ctx)
	assert.NilError(t, err)
	slices.SortFunc(got, netip.Addr.Compare)
	slices.SortFunc(want, netip.Addr.Compare)
	assert.Assert(t, slices.Equal(got, want), "got %v, want %v", got, want)
}
