package docker_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop", false)
	assert.NilError(t, err)

	c1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ContainerRemove(context.Background(), c1.ID, client.ContainerRemoveOptions{Force: true})
	})

	i1, err := dind.client.ContainerInspect(ctx, c1.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	c2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ContainerRemove(context.Background(), c2.ID, client.ContainerRemoveOptions{Force: true})
	})

	i2, err := dind.client.ContainerInspect(ctx, c2.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
		{
			Name:    strings.TrimPrefix(i1.Container.Name, "/"),
			Group:   "default",
			Enabled: "true",
		},
		{
			Name:    strings.TrimPrefix(i2.Container.Name, "/"),
			Group:   "my-group",
			Enabled: "true",
		},
	}
	// Assert go is equal to want
	// Sort both array to ensure they are equal
	sort.Slice(got, func(i, j int) bool {
		return strings.Compare(got[i].Name, got[j].Name) < 0
	})
	sort.Slice(want, func(i, j int) bool {
		return strings.Compare(want[i].Name, want[j].Name) < 0
	})
	assert.DeepEqual(t, got, want)
}

func TestDockerClassicProvider_GetGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := sharedDinD
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop", false)
	assert.NilError(t, err)

	c1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ContainerRemove(context.Background(), c1.ID, client.ContainerRemoveOptions{Force: true})
	})

	i1, err := dind.client.ContainerInspect(ctx, c1.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	c2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_, _ = dind.client.ContainerRemove(context.Background(), c2.ID, client.ContainerRemoveOptions{Force: true})
	})

	i2, err := dind.client.ContainerInspect(ctx, c2.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default":  {strings.TrimPrefix(i1.Container.Name, "/")},
		"my-group": {strings.TrimPrefix(i2.Container.Name, "/")},
	}

	assert.DeepEqual(t, got, want)
}
