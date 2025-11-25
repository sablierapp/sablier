package dockerswarm_test

import (
	"github.com/moby/moby/api/types/swarm"
	"github.com/sablierapp/sablier/pkg/sablier"

	"sort"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := setupDinD(t)
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	s1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, _, err := dind.client.ServiceInspectWithRaw(ctx, s1.ID, swarm.ServiceInspectOptions{})
	assert.NilError(t, err)

	s2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, _, err := dind.client.ServiceInspectWithRaw(ctx, s2.ID, swarm.ServiceInspectOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
		{
			Name:  i1.Spec.Name,
			Group: "default",
		},
		{
			Name:  i2.Spec.Name,
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
	dind := setupDinD(t)
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	s1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1, _, err := dind.client.ServiceInspectWithRaw(ctx, s1.ID, swarm.ServiceInspectOptions{})
	assert.NilError(t, err)

	s2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2, _, err := dind.client.ServiceInspectWithRaw(ctx, s2.ID, swarm.ServiceInspectOptions{})
	assert.NilError(t, err)

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default":  {i1.Spec.Name},
		"my-group": {i2.Spec.Name},
	}

	assert.DeepEqual(t, got, want)
}
