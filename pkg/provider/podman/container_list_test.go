package podman_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestPodmanProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	pind := setupPinD(t)
	p, err := podman.New(ctx, pind.client, slogt.New(t))
	assert.NilError(t, err)

	c1, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, err := pind.client.ContainerInspect(ctx, c1.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	c2, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, err := pind.client.ContainerInspect(ctx, c2.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
		{
			Name:  strings.TrimPrefix(i1.Container.Name, "/"),
			Group: "default",
		},
		{
			Name:  strings.TrimPrefix(i2.Container.Name, "/"),
			Group: "my-group",
		},
	}
	// Sort both slices to ensure order-independent comparison
	sort.Slice(got, func(i, j int) bool {
		return strings.Compare(got[i].Name, got[j].Name) < 0
	})
	sort.Slice(want, func(i, j int) bool {
		return strings.Compare(want[i].Name, want[j].Name) < 0
	})
	assert.DeepEqual(t, got, want)
}

func TestPodmanProvider_GetGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	pind := setupPinD(t)
	p, err := podman.New(ctx, pind.client, slogt.New(t))
	assert.NilError(t, err)

	c1, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, err := pind.client.ContainerInspect(ctx, c1.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	c2, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, err := pind.client.ContainerInspect(ctx, c2.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default":  {strings.TrimPrefix(i1.Container.Name, "/")},
		"my-group": {strings.TrimPrefix(i2.Container.Name, "/")},
	}

	assert.DeepEqual(t, got, want)
}
