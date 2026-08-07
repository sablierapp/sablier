package docker_test

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

	//nolint:staticcheck // Ignore "SA9003: Need https://github.com/testcontainers/testcontainers-go/pull/3672 to be released
	if err = c.LoadImage(ctx, "sablierapp/mimic:v0.3.3"); err != nil {
		// _ = c.Terminate(ctx)
		// log.Fatalf("failed to load mimic image: %v", err)
	}

	sharedDinD = &dindContainer{
		Container: c,
		client:    dindCli,
	}

	// Enable the cgroupv2 "io" controller inside the DinD container so that blkio
	// resource updates (BlkioWeight, device throttle) work for nested containers.
	//
	// How it works:
	//  - The DinD container already runs with --privileged and --cgroupns=host,
	//    so it sees and can write to the host VM's cgroup hierarchy.
	//  - The inner dockerd creates /sys/fs/cgroup/docker/ when it starts.
	//    For containers created under that cgroup to have io.weight / io.max,
	//    "io" must be present in /sys/fs/cgroup/docker/cgroup.subtree_control.
	//  - Writing "+io" to the root subtree_control makes the controller
	//    available to child cgroups; writing it to the docker subtree_control
	//    makes it available to container cgroups the inner dockerd creates.
	//
	// This is a best-effort step: if the underlying kernel or VM doesn't support
	// the io controller (e.g. cgroupv1, restricted Docker Desktop VM), the exec
	// will fail silently and the blkio tests will skip themselves at runtime.
	enableIO := "echo +io > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null; " +
		"echo +io > /sys/fs/cgroup/docker/cgroup.subtree_control 2>/dev/null; true"
	if exitCode, _, execErr := c.Exec(ctx, []string{"sh", "-c", enableIO}); execErr != nil || exitCode != 0 {
		log.Printf("note: could not enable cgroupv2 io controller in DinD (code=%d, err=%v); blkio tests will be skipped", exitCode, execErr)
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
