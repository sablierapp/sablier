package docker_test

import (
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
	"sort"
	"strings"
	"testing"
)

func TestDockerClassicProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := setupDinD(t, ctx)
	p, err := docker.NewDockerClassicProvider(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, err := dind.client.ContainerInspect(ctx, c1.ID)
	assert.NilError(t, err)

	assert.NilError(t, err)

	c2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, err := dind.client.ContainerInspect(ctx, c2.ID)
	assert.NilError(t, err)

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []types.Instance{
		{
			Name:  strings.TrimPrefix(i1.Name, "/"),
			Group: "default",
		},
		{
			Name:  strings.TrimPrefix(i2.Name, "/"),
			Group: "my-group",
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
	dind := setupDinD(t, ctx)
	p, err := docker.NewDockerClassicProvider(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	c1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, err := dind.client.ContainerInspect(ctx, c1.ID)
	assert.NilError(t, err)

	assert.NilError(t, err)

	c2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, err := dind.client.ContainerInspect(ctx, c2.ID)
	assert.NilError(t, err)

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default":  {strings.TrimPrefix(i1.Name, "/")},
		"my-group": {strings.TrimPrefix(i2.Name, "/")},
	}

	assert.DeepEqual(t, got, want)
}
