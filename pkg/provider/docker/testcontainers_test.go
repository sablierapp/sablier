package docker_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"slices"
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
	/*i, err := d.client.ImagePull(ctx, "docker.io/sablierapp/mimic:v0.3.1", image.PullOptions{})
	if err != nil {
		return container.CreateResponse{}, err
	}
	_, err = d.client.ImageLoad(ctx, i, false)
	if err != nil {
		return container.CreateResponse{}, err
	}*/

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
			Image:  "sablierapp/mimic:v0.3.1",
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
		Image:  "sablierapp/mimic:v0.3.1",
		Labels: labels,
	}, nil, nil, nil, opts.Name)
}

func setupDinD(t *testing.T, ctx context.Context) (*dindContainer, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

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
	t.Cleanup(func() {
		testcontainers.CleanupContainer(t, c)
	})

	ip, err := c.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := c.MappedPort(ctx, "2375")
	if err != nil {
		return nil, err
	}

	host := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	dindCli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker in docker client: %w", err)
	}

	err = addMimicToDind(ctx, cli, dindCli)
	if err != nil {
		return nil, fmt.Errorf("failed to add mimic to dind: %w", err)
	}

	return &dindContainer{
		Container: c,
		client:    dindCli,
	}, nil
}

func searchMimicImage(ctx context.Context, cli *client.Client) (string, error) {
	images, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list images: %w", err)
	}

	for _, summary := range images {
		if slices.Contains(summary.RepoTags, "sablierapp/mimic:v0.3.1") {
			return summary.ID, nil
		}
	}

	return "", nil
}

func pullMimicImage(ctx context.Context, cli *client.Client) error {
	reader, err := cli.ImagePull(ctx, "sablierapp/mimic:v0.3.1", image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	resp, err := cli.ImageLoad(ctx, reader, true)
	if err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func addMimicToDind(ctx context.Context, cli *client.Client, dindCli *client.Client) error {
	ID, err := searchMimicImage(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to search for mimic image: %w", err)
	}

	if ID == "" {
		err = pullMimicImage(ctx, cli)
		if err != nil {
			return err
		}

		ID, err = searchMimicImage(ctx, cli)
		if err != nil {
			return fmt.Errorf("failed to search for mimic image even though it's just been pulled without errors: %w", err)
		}
	}

	reader, err := cli.ImageSave(ctx, []string{ID})
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}
	defer reader.Close()

	resp, err := dindCli.ImageLoad(ctx, reader, true)
	if err != nil {
		return fmt.Errorf("failed to load image in docker in docker container: %w", err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read from response body: %w", err)
	}

	list, err := dindCli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return err
	}

	err = dindCli.ImageTag(ctx, list[0].ID, "sablierapp/mimic:v0.3.1")
	if err != nil {
		return err
	}

	return nil
}
