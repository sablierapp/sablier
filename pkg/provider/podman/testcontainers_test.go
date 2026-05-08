package podman_test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/testcontainers/pind"
	"github.com/testcontainers/testcontainers-go"
)

// sharedPinD is the single Podman-in-Docker container shared across all tests in this package.
// It is initialized by TestMain, which avoids the overhead of starting a new PinD per test.
var sharedPinD *pindContainer

type pindContainer struct {
	testcontainers.Container
	client *client.Client
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

	return d.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Entrypoint:  opts.Cmd,
			Image:       "docker.io/sablierapp/mimic:v0.3.3",
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

	c, err := pind.Run(ctx, "quay.io/podman/stable:v5.8.2")
	if err != nil {
		log.Fatalf("failed to start PinD: %v", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to get PinD host: %v", err)
	}

	pindCli, err := client.New(client.WithHost(host))
	if err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to create podman client: %v", err)
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
		_ = c.Terminate(ctx)
		log.Fatalf("failed to load mimic image: %v", err)
	}

	sharedPinD = &pindContainer{
		Container: c,
		client:    pindCli,
	}

	code := m.Run()
	_ = c.Terminate(ctx)
	os.Exit(code)
}

// WaitForContainerHealth polls ContainerInspect until the container's health status matches wantStatus.
func WaitForContainerHealth(ctx context.Context, cli *client.Client, id, wantStatus string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled waiting for container %s health status %q", id, wantStatus)
		case <-ticker.C:
			info, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
			if err != nil {
				return fmt.Errorf("error inspecting container: %w", err)
			}
			if info.Container.State.Health != nil && string(info.Container.State.Health.Status) == wantStatus {
				return nil
			}
		}
	}
}

// WaitForContainerRunning polls ContainerInspect until the container is running.
func WaitForContainerRunning(ctx context.Context, cli *client.Client, id string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled waiting for container %s to be running", id)
		case <-ticker.C:
			info, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
			if err != nil {
				return fmt.Errorf("error inspecting container: %w", err)
			}
			if info.Container.State.Status == "running" {
				return nil
			}
		}
	}
}
