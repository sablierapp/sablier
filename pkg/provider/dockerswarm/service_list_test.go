package dockerswarm_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	s1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1Result, err := dind.client.ServiceInspect(ctx, s1.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	i1 := i1Result.Service
	t.Cleanup(func() {
		_, _ = sharedDinD.client.ServiceRemove(context.Background(), i1.ID, client.ServiceRemoveOptions{})
	})

	s2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2Result, err := dind.client.ServiceInspect(ctx, s2.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	i2 := i2Result.Service
	t.Cleanup(func() {
		_, _ = sharedDinD.client.ServiceRemove(context.Background(), i2.ID, client.ServiceRemoveOptions{})
	})

	got, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All: true,
	})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
		{
			Name:    i1.Spec.Name,
			Groups:  []string{"default"},
			Enabled: "true",
		},
		{
			Name:    i2.Spec.Name,
			Groups:  []string{"my-group"},
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
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	s1, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	i1Result, err := dind.client.ServiceInspect(ctx, s1.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	i1 := i1Result.Service
	t.Cleanup(func() {
		_, _ = sharedDinD.client.ServiceRemove(context.Background(), i1.ID, client.ServiceRemoveOptions{})
	})

	s2, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "my-group",
		},
	})
	assert.NilError(t, err)

	i2Result, err := dind.client.ServiceInspect(ctx, s2.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	i2 := i2Result.Service
	t.Cleanup(func() {
		_, _ = sharedDinD.client.ServiceRemove(context.Background(), i2.ID, client.ServiceRemoveOptions{})
	})

	got, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	want := map[string][]string{
		"default":  {i1.Spec.Name},
		"my-group": {i2.Spec.Name},
	}

	assert.DeepEqual(t, got, want)
}

func TestDockerSwarmProvider_InstanceList_OnlyRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	dind := sharedDinD
	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t))
	assert.NilError(t, err)

	running, err := dind.CreateMimic(ctx, MimicOptions{
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	runningResult, err := dind.client.ServiceInspect(ctx, running.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	runningService := runningResult.Service
	t.Cleanup(func() {
		_, _ = sharedDinD.client.ServiceRemove(context.Background(), runningService.ID, client.ServiceRemoveOptions{})
	})

	var zero uint64
	scaledToZero, err := dind.CreateMimic(ctx, MimicOptions{
		Replicas: &zero,
		Labels: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	scaledResult, err := dind.client.ServiceInspect(ctx, scaledToZero.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	scaledService := scaledResult.Service
	t.Cleanup(func() {
		_, _ = sharedDinD.client.ServiceRemove(context.Background(), scaledService.ID, client.ServiceRemoveOptions{})
	})

	err = WaitForServiceRunning(ctx, dind.client, runningService.Spec.Name, 1)
	assert.NilError(t, err)

	// All includes the scaled-to-zero service
	got, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.Equal(t, len(got), 2)

	// Only running excludes the scaled-to-zero service
	got, err = p.InstanceList(ctx, provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Name, runningService.Spec.Name)
}
