package dockerswarm_test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/dind"
)

// sharedDinD is the single Docker-in-Docker Swarm container shared across all tests in this package.
// It is initialized by TestMain, which avoids the overhead of starting a new DinD per test.
var sharedDinD *dindContainer

type dindContainer struct {
	testcontainers.Container
	client *client.Client
}

type MimicOptions struct {
	Cmd           []string
	Healthcheck   *container.HealthConfig
	RestartPolicy *swarm.RestartPolicy
	Labels        map[string]string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (client.ServiceCreateResult, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	var replicas uint64 = 1
	return d.client.ServiceCreate(ctx, client.ServiceCreateOptions{
		Spec: swarm.ServiceSpec{
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{Replicas: &replicas},
			},
			TaskTemplate: swarm.TaskSpec{
				RestartPolicy: opts.RestartPolicy,
				ContainerSpec: &swarm.ContainerSpec{
					Image:       "sablierapp/mimic:v0.3.3",
					Healthcheck: opts.Healthcheck,
					Command:     opts.Cmd,
				},
			},
			Annotations: swarm.Annotations{
				Labels: opts.Labels,
			},
		},
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

	// Initialize the swarm
	if _, err = dindCli.SwarmInit(ctx, client.SwarmInitOptions{ListenAddr: "0.0.0.0"}); err != nil {
		_ = c.Terminate(ctx)
		log.Fatalf("failed to initialize swarm: %v", err)
	}

	sharedDinD = &dindContainer{
		Container: c,
		client:    dindCli,
	}

	code := m.Run()
	_ = c.Terminate(ctx)
	os.Exit(code)
}

// WaitForServiceRunning polls until the named service has the expected number of running tasks.
func WaitForServiceRunning(ctx context.Context, cli *client.Client, name string, runningTasks uint64) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for service %s to have %d running tasks", name, runningTasks)
		case <-ticker.C:
			filters := client.Filters{}
			filters.Add("name", name)
			services, err := cli.ServiceList(ctx, client.ServiceListOptions{Filters: filters, Status: true})
			if err != nil {
				return fmt.Errorf("error listing services: %w", err)
			}
			for _, svc := range services.Items {
				if svc.Spec.Name == name && svc.ServiceStatus != nil && svc.ServiceStatus.RunningTasks >= runningTasks {
					return nil
				}
			}
		}
	}
}
