package docker

import (
	"context"
	"github.com/docker/docker/api/types/checkpoint"
	"github.com/docker/docker/api/types/container"
)

func (d *DockerProvider) Stop(ctx context.Context, name string) error {
	if d.UsePause {
		return d.Client.ContainerPause(ctx, name)
	}

	if d.UseCheckpoint {
		return d.Client.CheckpointCreate(ctx, name, checkpoint.CreateOptions{
			CheckpointID: name,
			Exit:         true,
		})
	}

	return d.Client.ContainerStop(ctx, name, container.StopOptions{})
}
