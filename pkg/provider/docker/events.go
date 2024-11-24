package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/sablier"
	"time"
)

func (d *DockerProvider) AfterReady(ctx context.Context, name string) <-chan error {
	ch := make(chan error, 1)

	go func() {
		c, err := d.Client.ContainerInspect(ctx, name)
		if err != nil {
			ch <- err
			return
		}

		action := events.ActionStart
		if c.Config.Healthcheck != nil {
			d.log.Trace().Str("name", name).Msg("container has healthcheck, will be waiting for \"health_status: healthy\"")
			action = events.ActionHealthStatusHealthy
		} else {
			d.log.Trace().Str("name", name).Msg("container has no healthcheck, will be waiting for \"start\"")
		}

		ready := d.afterAction(ctx, name, action)
		ticker := time.NewTicker(5 * time.Second)

		for {
			select {
			case <-ctx.Done():
				ch <- ctx.Err()
				return
			case <-ticker.C:
				info, err := d.Info(ctx, name)
				if err != nil {
					ch <- ctx.Err()
					return
				}
				if info.Status == sablier.InstanceReady {
					ch <- nil
					return
				}
			case err = <-ready:
				ch <- err
				return
			}
		}
	}()

	return ch
}

func (d *DockerProvider) afterAction(ctx context.Context, name string, action events.Action) <-chan error {
	ch := make(chan error, 1)

	msgs, errs := d.Client.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "local"),
			filters.Arg("type", string(events.ContainerEventType)),
			filters.Arg("container", name),
			filters.Arg("event", string(action)),
		),
	})

	go func() {
		for {
			select {
			case <-ctx.Done():
				ch <- ctx.Err()
				return
			case msg, ok := <-msgs:
				if !ok {
					ch <- fmt.Errorf("events channel closed")
					return
				}
				d.log.Trace().Str("name", name).Any("event", msg).Msg("event received")
				ch <- nil
				return
			case err, ok := <-errs:
				if !ok {
					ch <- fmt.Errorf("events channel closed")
				}
				ch <- err
				return
			}
		}
	}()

	return ch
}
