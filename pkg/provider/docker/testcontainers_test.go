package docker_test

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"testing"
	"time"
)

type dindContainer struct {
	testcontainers.Container
	client *client.Client
}

type MimicOptions struct {
	Name         string
	WithHealth   bool
	HealthyAfter time.Duration
	RunningAfter time.Duration
	Registered   bool
	SablierGroup string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (container.CreateResponse, error) {
	i, err := d.client.ImagePull(ctx, "docker.io/sablierapp/mimic:v0.3.1", image.PullOptions{})
	if err != nil {
		return container.CreateResponse{}, err
	}
	_, err = d.client.ImageLoad(ctx, i, false)
	if err != nil {
		return container.CreateResponse{}, err
	}

	labels := make(map[string]string)
	if opts.Registered == true {
		labels["sablier.enable"] = "true"
		if opts.SablierGroup != "" {
			labels["sablier.group"] = opts.SablierGroup
		}
	}

	if opts.WithHealth == false {
		return d.client.ContainerCreate(ctx, &container.Config{
			Cmd:    []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy=false"},
			Image:  "docker.io/sablierapp/mimic:v0.3.1",
			Labels: labels,
		}, nil, nil, nil, opts.Name)
	}
	return d.client.ContainerCreate(ctx, &container.Config{
		Cmd: []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy", "--healthy-after", opts.HealthyAfter.String()},
		Healthcheck: &container.HealthConfig{
			Test:          []string{"CMD", "/mimic", "healthcheck"},
			Interval:      100 * time.Millisecond,
			Timeout:       time.Second,
			StartPeriod:   opts.RunningAfter,
			StartInterval: time.Second,
			Retries:       50,
		},
		Image:  "docker.io/sablierapp/mimic:v0.3.1",
		Labels: labels,
	}, nil, nil, nil, opts.Name)
}

func setupDinD(t *testing.T, ctx context.Context) (*dindContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "docker:dind",
		ExposedPorts: []string{"2375/tcp"},
		WaitingFor:   wait.ForLog("API listen on [::]:2375"),
		Cmd: []string{
			"dockerd", "-H", "tcp://0.0.0.0:2375", "--tls=false",
		},
		Privileged: true,
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           testcontainers.TestLogger(t),
	})
	if err != nil {
		return nil, err
	}

	ip, err := c.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := c.MappedPort(ctx, "2375")
	if err != nil {
		return nil, err
	}

	// DOCKER_HOST
	host := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	fmt.Println("DOCKER_HOST: ", host)
	cli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &dindContainer{
		Container: c,
		client:    cli,
	}, nil
}
