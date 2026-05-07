package docker_test

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/dind"
)

// sharedDinD is the single Docker-in-Docker container shared across all tests in this package.
// It is initialized by TestMain, which avoids the overhead of starting a new DinD per test.
var sharedDinD *dindContainer

type dindContainer struct {
	testcontainers.Container
	client *client.Client
}

type MimicOptions struct {
	Cmd           []string
	Healthcheck   *container.HealthConfig
	RestartPolicy container.RestartPolicy
	Labels        map[string]string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (client.ContainerCreateResult, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	return d.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Entrypoint:  opts.Cmd,
			Image:       "sablierapp/mimic:v0.3.3",
			Labels:      opts.Labels,
			Healthcheck: opts.Healthcheck,
		},
		HostConfig: &container.HostConfig{RestartPolicy: opts.RestartPolicy},
	})
}

func TestMain(m *testing.M) {
	// flag.Parse must be called before testing.Short() is usable.
	flag.Parse()

	// Skip the expensive container setup when running in short mode.
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()

	c, err := dind.Run(ctx, "docker:28.0.4-dind")
	if err != nil {
		log.Fatalf("failed to start DinD: %v", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to get DinD host: %v", err)
	}

	dindCli, err := client.New(client.WithHost(host))
	if err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to create docker client: %v", err)
	}

	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to get docker provider: %v", err)
	}

	if err = provider.PullImage(ctx, "sablierapp/mimic:v0.3.3"); err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to pull mimic image: %v", err)
	}

	if err = c.LoadImage(ctx, "sablierapp/mimic:v0.3.3"); err != nil {
		// _ = c.Terminate(ctx)
		// log.Fatalf("failed to load mimic image: %v", err)
	}

	sharedDinD = &dindContainer{
		Container: c,
		client:    dindCli,
	}

	code := m.Run()
	_ = c.Terminate(ctx)
	os.Exit(code)
}
