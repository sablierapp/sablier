package nomad_test

import (
	"context"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"gotest.tools/v3/assert"
)

func TestNomadProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	nc := setupNomad(t)

	p, err := nomad.New(ctx, nc.client, "default", slogt.New(t))
	assert.NilError(t, err)

	// Create a job with 1 allocation
	job, err := nc.CreateMimicJob(ctx, MimicJobOptions{
		Count: 1,
		Meta: map[string]string{
			"sablier.enable": "true",
		},
	})
	assert.NilError(t, err)

	instanceName := formatJobName(*job.ID, *job.TaskGroups[0].Name)

	// Wait for allocation to be running
	err = WaitForJobAllocations(ctx, nc.client, *job.ID, *job.TaskGroups[0].Name, 1)
	assert.NilError(t, err)

	// Start watching for stop events
	stoppedChan := make(chan string, 1)
	go p.NotifyInstanceStopped(ctx, stoppedChan)

	// Give the watcher time to initialize
	time.Sleep(2 * time.Second)

	// Scale the job to 0
	err = p.InstanceStop(ctx, instanceName)
	assert.NilError(t, err)

	// Wait for the notification
	select {
	case name := <-stoppedChan:
		assert.Equal(t, instanceName, name)
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for instance stopped notification")
	}
}
