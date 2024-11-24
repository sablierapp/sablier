package docker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/stretchr/testify/assert"
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
	WithHealth   bool
	HealthyAfter time.Duration
	RunningAfter time.Duration
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

	if opts.WithHealth == false {
		return d.client.ContainerCreate(ctx, &container.Config{
			Cmd:   []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy=false"},
			Image: "docker.io/sablierapp/mimic:v0.3.1",
			Labels: map[string]string{
				"sablier.enable": "true",
				"sablier.group":  opts.SablierGroup,
			},
		}, nil, nil, nil, "")
	}
	return d.client.ContainerCreate(ctx, &container.Config{
		Cmd: []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy", "--healthy-after", opts.HealthyAfter.String()},
		Healthcheck: &container.HealthConfig{
			Test:          []string{"CMD", "/mimic", "healthcheck"},
			Interval:      time.Second,
			Timeout:       3 * time.Second,
			StartPeriod:   opts.RunningAfter,
			StartInterval: time.Second,
			Retries:       15,
		},
		Image: "docker.io/sablierapp/mimic:v0.3.1",
		Labels: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  opts.SablierGroup,
		},
	}, nil, nil, nil, "")
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

func TestDockerProvider_StartWithHealthcheck(t *testing.T) {
	ctx := context.Background()
	dind, err := setupDinD(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := docker.NewDockerProvider(dind.client)
	if err != nil {
		t.Fatal(err)
	}

	mimic, err := dind.CreateMimic(ctx, MimicOptions{
		WithHealth:   true,
		HealthyAfter: 2 * time.Second,
		RunningAfter: 1 * time.Second,
		SablierGroup: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = p.Start(ctx, mimic.ID, provider.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	inspect, err := dind.client.ContainerInspect(ctx, mimic.ID)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := json.Marshal(inspect)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("inspect: %+v\n", string(resp))
	assert.Equal(t, inspect.State.Status, "running")
	assert.Equal(t, inspect.State.Health.Status, "healthy")
}

func TestDockerProvider_StartWithoutHealthcheck(t *testing.T) {
	ctx := context.Background()
	dind, err := setupDinD(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := docker.NewDockerProvider(dind.client)
	if err != nil {
		t.Fatal(err)
	}

	mimic, err := dind.CreateMimic(ctx, MimicOptions{
		WithHealth:   false,
		RunningAfter: 1 * time.Second,
		SablierGroup: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = p.Start(ctx, mimic.ID, provider.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	inspect, err := dind.client.ContainerInspect(ctx, mimic.ID)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := json.Marshal(inspect)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("inspect: %+v\n", string(resp))
	assert.Equal(t, inspect.State.Status, "running")
}
