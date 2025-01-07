package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"time"
)

func (d *DockerProvider) Start(ctx context.Context, name string, opts provider.StartOptions) (err error) {
	instance, err := d.Info(ctx, name)
	if err != nil {
		return err
	}

	if instance.Status == sablier.InstanceReady {
		// <-time.After()
		// TODO: What to do with that ?
		return nil
	}

	if d.UsePause {
		err = d.Client.ContainerUnpause(ctx, name)
		if err != nil {
			return err
		}
	}

	if d.UseCheckpoint {
		// TODO: List checkpoints, if none, start without checkpoint id
		err = d.Client.ContainerStart(ctx, name, container.StartOptions{
			CheckpointID: name,
		})
		if err != nil {
			return err
		}
	}

	return d.start(ctx, name, opts)

}

func (d *DockerProvider) start(ctx context.Context, name string, opts provider.StartOptions) error {
	readyCh := d.AfterReady(ctx, name)

	err := d.Client.ContainerStart(ctx, name, container.StartOptions{})
	if err != nil {
		return err
	}

	err = <-readyCh
	if err != nil {
		return err
	}

	<-time.After(opts.ConsiderReadyAfter)

	return nil
}
