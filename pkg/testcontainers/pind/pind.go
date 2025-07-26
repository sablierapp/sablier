package pind

import (
	"context"
	"errors"
	"fmt"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
)

// Container represents the Podman in Docker container type used in the module
type Container struct {
	testcontainers.Container
}

// Run creates an instance of the Podman in Docker container type
func Run(ctx context.Context, img string, opts ...testcontainers.ContainerCustomizer) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image: img,
		ExposedPorts: []string{
			"34451/tcp",
		},
		ConfigModifier: func(config *container.Config) {
			config.User = "podman"
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
			hc.SecurityOpt = []string{"seccomp=unconfined", "label=disable"}
		},
		Cmd: []string{
			"podman", "--log-level", "debug", "system", "service", "tcp://0.0.0.0:34451", "-t", "0",
		},
		WaitingFor: wait.ForListeningPort("34451/tcp"),
	}

	genericContainerReq := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	for _, opt := range opts {
		if err := opt.Customize(&genericContainerReq); err != nil {
			return nil, err
		}
	}

	container, err := testcontainers.GenericContainer(ctx, genericContainerReq)
	var c *Container
	if container != nil {
		c = &Container{Container: container}
	}

	if err != nil {
		return c, fmt.Errorf("generic container: %w", err)
	}

	return c, nil
}

// Host returns the endpoint to connect to the Podman service running inside the PinD container.
func (c *Container) Host(ctx context.Context) (string, error) {
	return c.PortEndpoint(ctx, "34451/tcp", "tcp")
}

// LoadImage loads an image into the PinD container.
func (c *Container) LoadImage(ctx context.Context, image string) (err error) {
	var provider testcontainers.GenericProvider
	if provider, err = testcontainers.ProviderDocker.GetProvider(); err != nil {
		return fmt.Errorf("get docker provider: %w", err)
	}

	// save image
	imagesTar, err := os.CreateTemp(os.TempDir(), "image*.tar")
	if err != nil {
		return fmt.Errorf("create temporary images file: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.Remove(imagesTar.Name()))
	}()

	if err = provider.SaveImages(ctx, imagesTar.Name(), image); err != nil {
		return fmt.Errorf("save images: %w", err)
	}

	containerPath := "/image/" + filepath.Base(imagesTar.Name())
	if err = c.CopyFileToContainer(ctx, imagesTar.Name(), containerPath, 0o644); err != nil {
		return fmt.Errorf("copy image to container: %w", err)
	}

	if _, _, err = c.Exec(ctx, []string{"podman", "load", "-i", containerPath}); err != nil {
		return fmt.Errorf("import image: %w", err)
	}

	return nil
}
