package docker_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

// TestDockerClassicProvider_InstanceDependencies_ServiceCompletedSuccessfully
// verifies that InstanceDependencies resolves the Docker Compose depends_on label
// and returns the correct dependency with condition service_completed_successfully.
// See https://github.com/sablierapp/sablier/issues/792
func TestDockerClassicProvider_InstanceDependencies_ServiceCompletedSuccessfully(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	c := sharedDinD
	p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
	assert.NilError(t, err)

	const project = "depends-on-completed-test"

	migration, err := c.CreateMimic(ctx, MimicOptions{
		Cmd: []string{"/mimic", "-running=false", "-exit-code=0"},
		Labels: map[string]string{
			"com.docker.compose.project": project,
			"com.docker.compose.service": "migration",
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

	// Inspect migration to get its canonical container name (Docker strips the
	// leading "/" from Names[0] when comparing with the dep returned by the provider).
	migrationSpec, err := c.client.ContainerInspect(ctx, migration.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	migrationName := strings.TrimPrefix(migrationSpec.Container.Name, "/")

	deps, err := p.InstanceDependencies(ctx, app.ID)
	assert.NilError(t, err)
	assert.Equal(t, len(deps), 1, "expected one dependency")
	assert.Equal(t, deps[0].Name, migrationName)
	assert.Equal(t, deps[0].Condition, "service_completed_successfully")
}

// TestDockerClassicProvider_InstanceDependencies_ServiceHealthy verifies that a
// service_healthy depends_on is resolved and returned with the correct condition.
func TestDockerClassicProvider_InstanceDependencies_ServiceHealthy(t *testing.T) {
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

	// Inspect db to get its canonical container name.
	dbSpec, err := c.client.ContainerInspect(ctx, db.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	dbName := strings.TrimPrefix(dbSpec.Container.Name, "/")

	deps, err := p.InstanceDependencies(ctx, app.ID)
	assert.NilError(t, err)
	assert.Equal(t, len(deps), 1, "expected one dependency")
	assert.Equal(t, deps[0].Name, dbName)
	assert.Equal(t, deps[0].Condition, "service_healthy")
}

