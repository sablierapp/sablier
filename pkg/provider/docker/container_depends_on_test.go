package docker_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

// TestDockerClassicProvider_StartWithDependsOn verifies that starting a
// container resolves its Docker Compose depends_on dependencies first, respecting
// the declared conditions.
// See https://github.com/sablierapp/sablier/issues/792
func TestDockerClassicProvider_StartWithDependsOn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD
	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	const project = "depends-on-test"

	// migration is an init container that runs once and exits successfully.
	migration, err := c.CreateMimic(ctx, MimicOptions{
		Cmd: []string{"/mimic", "-running=false", "-exit-code=0"},
		Labels: map[string]string{
			"com.docker.compose.project": project,
			"com.docker.compose.service": "migration",
			"sablier.enable":             "true",
			"sablier.group":              project,
		},
	})
	assert.NilError(t, err)

	// app depends on migration having completed successfully before it starts.
	app, err := c.CreateMimic(ctx, MimicOptions{
		Cmd: []string{"/mimic", "-running", "-running-after=1ms"},
		Labels: map[string]string{
			"com.docker.compose.project":    project,
			"com.docker.compose.service":    "app",
			"com.docker.compose.depends_on": "migration:service_completed_successfully:false",
			"sablier.enable":                "true",
			"sablier.group":                 project,
		},
	})
	assert.NilError(t, err)

	startCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err = p.InstanceStart(startCtx, app.ID)
	assert.NilError(t, err)

	// The migration dependency must have been started and run to completion.
	migrationSpec, err := c.client.ContainerInspect(ctx, migration.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, migrationSpec.Container.State.Status, container.StateExited)
	assert.Equal(t, migrationSpec.Container.State.ExitCode, 0)

	// The app must be running once its dependency completed.
	appSpec, err := c.client.ContainerInspect(ctx, app.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, appSpec.Container.State.Running, true)
}

// TestDockerClassicProvider_StartWithDependsOnHealthy verifies that a
// service_healthy dependency is started and awaited until healthy before the
// dependent container is started.
func TestDockerClassicProvider_StartWithDependsOnHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD
	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	const project = "depends-on-healthy-test"

	db, err := c.CreateMimic(ctx, MimicOptions{
		Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy", "-healthy-after=1ms"},
		Healthcheck: &container.HealthConfig{
			Test:          []string{"CMD", "/mimic", "healthcheck"},
			Interval:      100 * time.Millisecond,
			Timeout:       time.Second,
			StartPeriod:   time.Second,
			StartInterval: 100 * time.Millisecond,
			Retries:       10,
		},
		Labels: map[string]string{
			"com.docker.compose.project": project,
			"com.docker.compose.service": "db",
		},
	})
	assert.NilError(t, err)

	app, err := c.CreateMimic(ctx, MimicOptions{
		Cmd: []string{"/mimic", "-running", "-running-after=1ms"},
		Labels: map[string]string{
			"com.docker.compose.project":    project,
			"com.docker.compose.service":    "app",
			"com.docker.compose.depends_on": "db:service_healthy:false",
		},
	})
	assert.NilError(t, err)

	startCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err = p.InstanceStart(startCtx, app.ID)
	assert.NilError(t, err)

	dbSpec, err := c.client.ContainerInspect(ctx, db.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, dbSpec.Container.State.Running, true)
	assert.Assert(t, dbSpec.Container.State.Health != nil)
	assert.Equal(t, dbSpec.Container.State.Health.Status, container.Healthy)

	appSpec, err := c.client.ContainerInspect(ctx, app.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, appSpec.Container.State.Running, true)
}
