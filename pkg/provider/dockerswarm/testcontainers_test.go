package dockerswarm_test

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
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
	RestartPolicy *swarm.RestartPolicy
	Labels        map[string]string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (swarm.ServiceCreateResponse, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	d.t.Log("Creating mimic service with options", opts)
	var replicas uint64 = 1
	return d.client.ServiceCreate(ctx, swarm.ServiceSpec{
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{Replicas: &replicas},
		},
		TaskTemplate: swarm.TaskSpec{
			RestartPolicy: opts.RestartPolicy,
			ContainerSpec: &swarm.ContainerSpec{
				Image:       "sablierapp/mimic:v0.3.1",
				Healthcheck: opts.Healthcheck,
				Command:     opts.Cmd,
			},
		},
		Annotations: swarm.Annotations{
			Labels: opts.Labels,
		},
	}, types.ServiceCreateOptions{})
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

	// Initialize the swarm
	_, err = dindCli.SwarmInit(ctx, swarm.InitRequest{
		ListenAddr: "0.0.0.0",
	})
	assert.NilError(t, err)

	return &dindContainer{
		Container: c,
		client:    dindCli,
		t:         t,
	}
}
