package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
)

func (p *DockerClassicProvider) Start(ctx context.Context, name string) error {
	// TODO: Start should block until the container is ready.
	err := p.Client.ContainerStart(ctx, name, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}
