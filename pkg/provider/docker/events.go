package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/sablier"
	"strconv"
	"time"
)

func (d *DockerProvider) Events(ctx context.Context) (<-chan sablier.Message, <-chan error) {
	ch := make(chan sablier.Message)
	errCh := make(chan error)
	started := make(chan struct{})

	go func() {
		defer close(ch)
		msgs, errs := d.Client.Events(ctx, events.ListOptions{
			Filters: filters.NewArgs(
				filters.Arg("scope", "local"),
				filters.Arg("type", string(events.ContainerEventType)),
			),
		})

		close(started)
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case msg, ok := <-msgs:
				if !ok {
					errCh <- fmt.Errorf("events channel closed")
					return
				}
				d.log.Trace().Any("event", msg).Msg("event received")
				e, ignore := d.parseEvent(ctx, msg)
				if !ignore {
					ch <- e
				} else {
					d.log.Trace().Any("event", msg).Msg("event ignored")
				}
			case err, ok := <-errs:
				if !ok {
					errCh <- fmt.Errorf("events channel closed")
					return
				}
				errCh <- err
				return
			}
		}
	}()
	<-started

	return ch, errCh
}

func (d *DockerProvider) parseEvent(ctx context.Context, message events.Message) (sablier.Message, bool) {
	instance := d.extractInstanceConfigFromEvent(message)

	var action sablier.EventAction
	switch message.Action {
	case events.ActionStart:
		spec, err := d.Client.ContainerInspect(ctx, instance.Name)
		action = sablier.EventActionStart
		if err == nil && spec.Config.Healthcheck != nil {
			action = sablier.EventActionCreate
		}
	case events.ActionHealthStatusHealthy:
		action = sablier.EventActionStart
	case events.ActionCreate:
		action = sablier.EventActionCreate
	case events.ActionDestroy:
		action = sablier.EventActionRemove
	case events.ActionDie:
		action = sablier.EventActionStop
	case events.ActionDelete:
		action = sablier.EventActionRemove
	case events.ActionKill:
		action = sablier.EventActionStop
	}

	return sablier.Message{
		Instance: instance,
		Action:   action,
	}, false
}

func (d *DockerProvider) extractInstanceConfigFromEvent(message events.Message) sablier.InstanceConfig {
	name := message.Actor.Attributes["name"]
	enabledLabel, ok := message.Actor.Attributes["sablier.enable"]
	if !ok {
		enabledLabel = "false"
	}

	if enabledLabel == "" {
		enabledLabel = "true"
	}

	enabled, err := strconv.ParseBool(enabledLabel)
	if err != nil {
		d.log.Warn().Err(err).Msg("unable to parse sablier.enable as a boolean")
		enabled = false
	}

	group, ok := message.Actor.Attributes["sablier.group"]
	if !ok {
		if enabled {
			group = name // Group defaults to the container name
		} else {
			group = "" // No group because not registered
		}
	}

	replicas, ok := message.Actor.Attributes["sablier.desired-replicas"]
	if !ok {
		replicas = "1"
	}

	desired, err := strconv.ParseUint(replicas, 10, 32)
	if err != nil {
		d.log.Warn().Err(err).Msg("unable to parse sablier.desired-replicas as a uint32")
		desired = 1
	}

	return sablier.InstanceConfig{
		Enabled:         enabled,
		Name:            name,
		Group:           group,
		DesiredReplicas: uint32(desired),
	}
}

func (d *DockerProvider) AfterReady(ctx context.Context, name string) <-chan error {
	ch := make(chan error, 1)
	started := make(chan struct{})

	go func() {
		defer close(ch)
		c, err := d.Client.ContainerInspect(ctx, name)
		if err != nil {
			close(started)
			ch <- err
			return
		}

		action := events.ActionStart
		if c.Config.Healthcheck != nil {
			d.log.Trace().Str("name", c.Name).Msg("container has healthcheck, will be waiting for \"health_status: healthy\"")
			action = events.ActionHealthStatusHealthy
		} else {
			d.log.Trace().Str("name", c.Name).Msg("container has no healthcheck, will be waiting for \"start\"")
		}

		ready := d.afterAction(ctx, name, action)
		ticker := time.NewTicker(5 * time.Second)

		close(started)
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
	<-started

	return ch
}

func (d *DockerProvider) afterAction(ctx context.Context, name string, action events.Action) <-chan error {
	ch := make(chan error, 1)
	started := make(chan struct{})

	go func() {
		defer close(ch)
		msgs, errs := d.Client.Events(ctx, events.ListOptions{
			Filters: filters.NewArgs(
				filters.Arg("scope", "local"),
				filters.Arg("type", string(events.ContainerEventType)),
				filters.Arg("container", name),
				filters.Arg("event", string(action)),
			),
		})

		close(started)
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
	<-started

	return ch
}
