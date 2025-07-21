package podman_test

import (
	"context"
	"testing"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/sablierapp/sablier/pkg/testcontainers/pind"
	"github.com/testcontainers/testcontainers-go"
	"gotest.tools/v3/assert"
)

type pindContainer struct {
	testcontainers.Container
	connText context.Context
	t        *testing.T
}

type MimicOptions struct {
	Cmd           []string
	Healthcheck   *manifest.Schema2HealthConfig
	RestartPolicy string
	Labels        map[string]string
}

func (d *pindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (entities.ContainerCreateResponse, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	d.t.Log("Creating mimic container with options", opts)
	// Container create
	s := specgen.NewSpecGenerator("sablierapp/mimic:v0.3.1", false)
	s.Labels = opts.Labels
	s.Entrypoint = opts.Cmd
	s.HealthConfig = opts.Healthcheck
	s.RestartPolicy = opts.RestartPolicy
	return containers.CreateWithSpec(ctx, s, nil)
}

func setupPinD(t *testing.T) *pindContainer {
	t.Helper()
	ctx := t.Context()
	c, err := pind.Run(ctx, "quay.io/podman/stable:v5.5.2")
	assert.NilError(t, err)
	testcontainers.CleanupContainer(t, c)

	host, err := c.Host(ctx)
	assert.NilError(t, err)

	connText, err := bindings.NewConnection(ctx, host)
	assert.NilError(t, err)

	provider, err := testcontainers.ProviderDocker.GetProvider()
	assert.NilError(t, err)

	err = provider.PullImage(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	err = c.LoadImage(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	return &pindContainer{
		Container: c,
		connText:  connText,
		t:         t,
	}
}
