package nomad_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type nomadContainer struct {
	container testcontainers.Container
	address   string
	client    *api.Client
	t         *testing.T
}

func setupNomad(t *testing.T) *nomadContainer {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "hashicorp/nomad:1.8",
		ExposedPorts: []string{"4646/tcp"},
		Cmd: []string{
			"agent",
			"-dev",
			"-bind=0.0.0.0",
			"-network-interface=eth0",
		},
		Privileged: true,
		WaitingFor: wait.ForLog("Nomad agent started!").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start nomad container: %s", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %s", err)
	}

	port, err := container.MappedPort(ctx, "4646")
	if err != nil {
		t.Fatalf("failed to get mapped port: %s", err)
	}

	address := fmt.Sprintf("http://%s:%s", host, port.Port())

	// Create Nomad client
	config := api.DefaultConfig()
	config.Address = address
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create nomad client: %s", err)
	}

	// Wait for Nomad to be ready
	time.Sleep(2 * time.Second)

	return &nomadContainer{
		container: container,
		address:   address,
		client:    client,
		t:         t,
	}
}

type MimicJobOptions struct {
	JobID         string
	TaskGroupName string
	Count         int
	Meta          map[string]string
	Cmd           []string
}

func (nc *nomadContainer) CreateMimicJob(ctx context.Context, opts MimicJobOptions) (*api.Job, error) {
	if opts.JobID == "" {
		opts.JobID = fmt.Sprintf("mimic-%d", time.Now().UnixNano())
	}
	if opts.TaskGroupName == "" {
		opts.TaskGroupName = "web"
	}
	if opts.Cmd == nil {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s"}
	}

	count := opts.Count
	job := &api.Job{
		ID:          &opts.JobID,
		Name:        &opts.JobID,
		Type:        stringToPtr("service"),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  &opts.TaskGroupName,
				Count: &count,
				Meta:  opts.Meta,
				Tasks: []*api.Task{
					{
						Name:   "mimic",
						Driver: "docker",
						Config: map[string]interface{}{
							"image":   "sablierapp/mimic:v0.3.1",
							"command": opts.Cmd[0],
							"args":    opts.Cmd[1:],
						},
						Resources: &api.Resources{
							CPU:      intToPtr(100),
							MemoryMB: intToPtr(128),
						},
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Attempts: intToPtr(0),
					Mode:     stringToPtr("fail"),
				},
			},
		},
	}

	jobs := nc.client.Jobs()
	_, _, err := jobs.Register(job, &api.WriteOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to register job: %w", err)
	}

	nc.t.Logf("Created Nomad job %s with task group %s", opts.JobID, opts.TaskGroupName)

	return job, nil
}

func stringToPtr(s string) *string {
	return &s
}

func intToPtr(i int) *int {
	return &i
}

func WaitForJobAllocations(ctx context.Context, client *api.Client, jobID string, taskGroup string, expectedCount int) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for job allocations")
		case <-timeout:
			return fmt.Errorf("timeout waiting for job allocations")
		case <-ticker.C:
			jobs := client.Jobs()
			allocations, _, err := jobs.Allocations(jobID, false, &api.QueryOptions{})
			if err != nil {
				return fmt.Errorf("error getting allocations: %w", err)
			}

			runningCount := 0
			for _, alloc := range allocations {
				if alloc.TaskGroup == taskGroup && alloc.ClientStatus == "running" {
					runningCount++
				}
			}

			if runningCount == expectedCount {
				return nil
			}
		}
	}
}

// formatJobName creates the instance name from job ID and task group name
func formatJobName(jobID string, taskGroupName string) string {
	return fmt.Sprintf("%s/%s", jobID, taskGroupName)
}
