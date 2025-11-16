package nomad_test

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"gotest.tools/v3/assert"
)

func TestNomadProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	nc := setupNomad(t)

	p, err := nomad.New(ctx, nc.client, "default", slogt.New(t))
	assert.NilError(t, err)

	// Create jobs with sablier.enable
	job1, err := nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "list-test-1",
		Count: 0,
		Meta: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "test-group",
		},
	})
	assert.NilError(t, err)

	job2, err := nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "list-test-2",
		Count: 0,
		Meta: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	// Job without sablier.enable
	_, err = nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "list-test-3",
		Count: 0,
	})
	assert.NilError(t, err)

	// Test with All = false (only enabled instances)
	instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.Equal(t, 2, len(instances))

	// Test with All = true (all instances)
	instancesAll, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.Assert(t, len(instancesAll) >= 3)

	// Verify instance configuration
	found := false
	for _, inst := range instances {
		if inst.Name == formatJobName(*job1.ID, *job1.TaskGroups[0].Name) {
			assert.Equal(t, "test-group", inst.Group)
			found = true
			break
		}
	}
	assert.Assert(t, found, "Expected to find job1 in instances list")

	// Verify default group
	foundDefault := false
	for _, inst := range instances {
		if inst.Name == formatJobName(*job2.ID, *job2.TaskGroups[0].Name) {
			assert.Equal(t, "default", inst.Group)
			foundDefault = true
			break
		}
	}
	assert.Assert(t, foundDefault, "Expected to find job2 with default group")
}
