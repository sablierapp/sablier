package podman_test

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/testcontainers/pind"
	"github.com/testcontainers/testcontainers-go"
	"gotest.tools/v3/assert"
)

type pindContainer struct {
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

func (d *pindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (client.ContainerCreateResult, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	d.t.Log("Creating mimic container with options", opts)
	return d.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Entrypoint:  opts.Cmd,
			Image:       "docker.io/sablierapp/mimic:v0.3.1",
			Labels:      opts.Labels,
			Healthcheck: opts.Healthcheck,
		},
		HostConfig: &container.HostConfig{RestartPolicy: opts.RestartPolicy},
	})
}

func setupPinD(t *testing.T) *pindContainer {
	t.Helper()
	ctx := t.Context()
	c, err := pind.Run(ctx, "quay.io/podman/stable:v5.8.2")
	assert.NilError(t, err)
	testcontainers.CleanupContainer(t, c)

	host, err := c.Host(ctx)
	assert.NilError(t, err)

	pindCli, err := client.New(client.WithHost(host))
	assert.NilError(t, err)

	provider, err := testcontainers.ProviderDocker.GetProvider()
	assert.NilError(t, err)

	err = provider.PullImage(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	err = c.LoadImage(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	return &pindContainer{
		Container: c,
		client:    pindCli,
		t:         t,
	}
}
