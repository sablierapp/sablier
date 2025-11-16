package nomad_test

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"gotest.tools/v3/assert"
)

func TestNomadProvider_InstanceGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	nc := setupNomad(t)

	p, err := nomad.New(ctx, nc.client, "default", slogt.New(t))
	assert.NilError(t, err)

	// Create jobs with different groups
	job1, err := nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "groups-test-1",
		Count: 0,
		Meta: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "group1",
		},
	})
	assert.NilError(t, err)

	job2, err := nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "groups-test-2",
		Count: 0,
		Meta: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "group1",
		},
	})
	assert.NilError(t, err)

	job3, err := nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "groups-test-3",
		Count: 0,
		Meta: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "group2",
		},
	})
	assert.NilError(t, err)

	// Job without sablier.enable should not be included
	_, err = nc.CreateMimicJob(ctx, MimicJobOptions{
		JobID: "groups-test-4",
		Count: 0,
	})
	assert.NilError(t, err)

	groups, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	assert.Equal(t, 2, len(groups))
	assert.Equal(t, 2, len(groups["group1"]))
	assert.Equal(t, 1, len(groups["group2"]))

	// Check that the instance names are correct
	expectedGroup1 := []string{
		formatJobName(*job1.ID, *job1.TaskGroups[0].Name),
		formatJobName(*job2.ID, *job2.TaskGroups[0].Name),
	}
	expectedGroup2 := []string{
		formatJobName(*job3.ID, *job3.TaskGroups[0].Name),
	}

	assert.DeepEqual(t, expectedGroup1, groups["group1"])
	assert.DeepEqual(t, expectedGroup2, groups["group2"])
}
