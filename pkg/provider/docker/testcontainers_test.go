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
	"gotest.tools/v3/assert"
	"slices"
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
	RestartPolicy container.RestartPolicy
	Labels        map[string]string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (container.CreateResponse, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	d.t.Log("Creating mimic container with options", opts)
	return d.client.ContainerCreate(ctx, &container.Config{
		Entrypoint:  opts.Cmd,
		Image:       "sablierapp/mimic:v0.3.1",
		Labels:      opts.Labels,
		Healthcheck: opts.Healthcheck,
	}, &container.HostConfig{RestartPolicy: opts.RestartPolicy}, nil, nil, "")
}

func setupDinD(t *testing.T, ctx context.Context) *dindContainer {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	assert.NilError(t, err)

	req := testcontainers.ContainerRequest{
		Image:        "docker:28.0.1-dind",
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
	assert.NilError(t, err)
	t.Cleanup(func() {
		testcontainers.CleanupContainer(t, c)
	})

	ip, err := c.Host(ctx)
	assert.NilError(t, err)

	mappedPort, err := c.MappedPort(ctx, "2375")
	assert.NilError(t, err)

	host := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	dindCli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	assert.NilError(t, err)

	err = addMimicToDind(ctx, cli, dindCli)
	assert.NilError(t, err)

	return &dindContainer{
		Container: c,
		client:    dindCli,
		t:         t,
	}
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
	resp, err := cli.ImageLoad(ctx, reader)
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

	resp, err := dindCli.ImageLoad(ctx, reader)
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
