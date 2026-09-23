package nomad_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/moby/moby/api/types/container"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// nomadContainer is a Nomad agent in development mode (server and client in
// one process) running in Docker. The test jobs run a plain sleep with raw_exec.
type nomadContainer struct {
	testcontainers.Container
	client *api.Client
}

// shared is the single Nomad agent shared by the integration tests of this
// package. It is started on first use and terminated by TestMain.
var (
	shared     *nomadContainer
	sharedOnce sync.Once
	sharedErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if shared != nil {
		_ = shared.Terminate(context.Background())
	}
	os.Exit(code)
}

func setupNomad(t *testing.T) *nomadContainer {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	sharedOnce.Do(func() {
		shared, sharedErr = startNomad(context.Background())
	})
	if sharedErr != nil {
		t.Fatalf("cannot start the Nomad agent: %v", sharedErr)
	}
	return shared
}

// rawExecConfig enables the raw_exec driver, which runs the test tasks
// directly in the agent container.
const rawExecConfig = `plugin "raw_exec" {
  config {
    enabled = true
  }
}
`

func startNomad(ctx context.Context) (*nomadContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "hashicorp/nomad:1.11.3",
		ExposedPorts: []string{"4646/tcp"},
		Cmd:          []string{"agent", "-dev", "-bind=0.0.0.0", "-network-interface=eth0", "-log-level=WARN", "-config=/etc/nomad.d/raw_exec.hcl"},
		Env:          map[string]string{"NOMAD_SKIP_DOCKER_IMAGE_WARN": "1"},
		Files: []testcontainers.ContainerFile{{
			Reader:            strings.NewReader(rawExecConfig),
			ContainerFilePath: "/etc/nomad.d/raw_exec.hcl",
			FileMode:          0o644,
		}},
		// The Nomad client creates its nomad.slice cgroup, so the container must be
		// privileged and see the host cgroup tree, like the testcontainers dind module.
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
			hc.CgroupnsMode = container.CgroupnsModeHost
		},
		WaitingFor: wait.ForHTTP("/v1/status/leader").WithPort("4646/tcp").WithStartupTimeout(2 * time.Minute),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot start container: %w", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		return nil, err
	}
	port, err := c.MappedPort(ctx, "4646/tcp")
	if err != nil {
		return nil, err
	}

	cfg := api.DefaultConfig()
	cfg.Address = fmt.Sprintf("http://%s:%s", host, port.Port())
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create nomad client: %w", err)
	}

	// Allocations can only be placed once the client node is ready.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		nodes, _, err := client.Nodes().List(nil)
		if err == nil {
			ready := false
			for _, n := range nodes {
				if n.Status == api.NodeStatusReady {
					ready = true
				}
			}
			if ready {
				break
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("no ready Nomad client node (last error: %v)", err)
		}
		time.Sleep(time.Second)
	}

	return &nomadContainer{Container: c, client: client}, nil
}

func (c *nomadContainer) provider(t *testing.T) *nomad.Provider {
	t.Helper()
	p, err := nomad.NewForTest(t.Context(), c.client, "default", slogt.New(t), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("cannot create provider: %v", err)
	}
	return p
}

// registerJob registers a service job with one task group "web" that runs a
// sleeping raw_exec task, purges it when the test ends and returns its instance name.
func (c *nomadContainer) registerJob(t *testing.T, id string, count int, meta map[string]string, minHealthy time.Duration) string {
	t.Helper()
	job := &api.Job{
		ID:          new(id),
		Name:        new(id),
		Type:        new(api.JobTypeService),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{{
			Name:  new("web"),
			Count: new(count),
			Meta:  meta,
			Update: &api.UpdateStrategy{
				HealthCheck:      new("task_states"),
				MinHealthyTime:   new(minHealthy),
				HealthyDeadline:  new(minHealthy + time.Minute),
				ProgressDeadline: new(minHealthy + 2*time.Minute),
			},
			Tasks: []*api.Task{{
				Name:   "sleep",
				Driver: "raw_exec",
				Config: map[string]any{
					"command": "/bin/sleep",
					"args":    []string{"3600"},
				},
				Resources: &api.Resources{CPU: new(20), MemoryMB: new(16)},
			}},
		}},
	}

	if _, _, err := c.client.Jobs().Register(job, nil); err != nil {
		t.Fatalf("cannot register job %q: %v", id, err)
	}
	t.Cleanup(func() {
		if _, _, err := c.client.Jobs().Deregister(id, true, nil); err != nil {
			t.Logf("cleanup: cannot purge job %q: %v", id, err)
		}
	})
	return id + "/web"
}
