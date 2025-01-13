package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"time"
)

func (d *DockerProvider) Start(ctx context.Context, name string, opts provider.StartOptions) (err error) {
	instance, spec, err := d.InfoWithSpec(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	if instance.Status == sablier.InstanceReady {
		// TODO: Info should not return ready if it does not respect the considerReadyAfter
		<-d.considerReadyAfter(spec, opts.ConsiderReadyAfter)
		return nil
	}

	if d.UsePause && spec.State.Paused {
		err = d.Client.ContainerUnpause(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to unpause container: %w", err)
		}
	}

	if d.UseCheckpoint && spec.State.Status == "exited" {
		// TODO: List checkpoints, if none, start without checkpoint id
		err = d.Client.ContainerStart(ctx, name, container.StartOptions{
			CheckpointID: name,
		})
		if err != nil {
			return fmt.Errorf("failed to start container with checkpoint: %w", err)
		}
	}

	return d.start(ctx, name, opts)

}

func (d *DockerProvider) start(ctx context.Context, name string, opts provider.StartOptions) error {
	readyCh := d.AfterReady(ctx, name, opts.ConsiderReadyAfter)

	err := d.Client.ContainerStart(ctx, name, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	err = <-readyCh
	if err != nil {
		return err
	}

	<-time.After(opts.ConsiderReadyAfter)

	return nil
}

func (d *DockerProvider) considerReadyAfter(spec types.ContainerJSON, considerReadyAfter time.Duration) <-chan time.Time {
	startedAt, err := time.Parse(time.RFC3339Nano, spec.State.StartedAt)
	if err != nil {
		d.log.Warn().Err(err).Str("name", spec.Name).Msg("unable to parse startedAt, will use current time")
		startedAt = time.Now()
	}

	elapsedTimeSinceReady := time.Since(startedAt)
	considerReadyAfter = considerReadyAfter - elapsedTimeSinceReady

	d.log.Trace().Str("name", spec.Name).Str("considerReadyAfter", considerReadyAfter.String()).Msg("waiting additional time for container to be considered ready")
	return time.After(considerReadyAfter)
}
