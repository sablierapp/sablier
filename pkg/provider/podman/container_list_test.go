package podman_test

import (
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	"sort"
	"strings"
	"testing"
)

func TestPodmanProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	pind := setupPinD(t)
	p, err := podman.New(pind.connText, slogt.New(t))
	assert.NilError(t, err)

	c1, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, err := containers.Inspect(pind.connText, c1.ID, nil)
	assert.NilError(t, err)

	assert.NilError(t, err)

	c2, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, err := containers.Inspect(pind.connText, c2.ID, nil)
	assert.NilError(t, err)

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
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
	pind := setupPinD(t)
	p, err := podman.New(pind.connText, slogt.New(t))
	assert.NilError(t, err)

	c1, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, err := containers.Inspect(pind.connText, c1.ID, nil)
	assert.NilError(t, err)

	assert.NilError(t, err)

	c2, err := pind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, err := containers.Inspect(pind.connText, c2.ID, nil)
	assert.NilError(t, err)

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default":  {strings.TrimPrefix(i1.Name, "/")},
		"my-group": {strings.TrimPrefix(i2.Name, "/")},
	}

	assert.DeepEqual(t, got, want)
}
