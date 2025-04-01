package docker_test

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/dind"
	"gotest.tools/v3/assert"
	"testing"
)

type dindContainer struct {
	testcontainers.Container
	client *client.Client
	t      *testing.T
}

type MimicOptions struct {
	Cmd           []string
	Healthcheck   *container.HealthConfig
	RestartPolicy container.RestartPolicy
	Labels        map[string]string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (container.CreateResponse, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	d.t.Log("Creating mimic container with options", opts)
	return d.client.ContainerCreate(ctx, &container.Config{
		Entrypoint:  opts.Cmd,
		Image:       "sablierapp/mimic:v0.3.1",
		Labels:      opts.Labels,
		Healthcheck: opts.Healthcheck,
	}, &container.HostConfig{RestartPolicy: opts.RestartPolicy}, nil, nil, "")
}

func setupDinD(t *testing.T) *dindContainer {
	t.Helper()
	ctx := t.Context()
	c, err := dind.Run(ctx, "docker:28.0.4-dind")
	assert.NilError(t, err)
	testcontainers.CleanupContainer(t, c)

	host, err := c.Host(ctx)
	assert.NilError(t, err)

	dindCli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	assert.NilError(t, err)

	provider, err := testcontainers.ProviderDocker.GetProvider()
	assert.NilError(t, err)

	err = provider.PullImage(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	err = c.LoadImage(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	return &dindContainer{
		Container: c,
		client:    dindCli,
		t:         t,
	}
}
