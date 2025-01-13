package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/sablier"
	"strconv"
	"time"
)

func (d *DockerProvider) Events(ctx context.Context) (<-chan sablier.Message, <-chan error) {
	ch := make(chan sablier.Message, 10)
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
	instance, spec, err := d.extractInstanceConfigFromEvent(ctx, message)
	if err != nil {
		d.log.Warn().Err(err).Any("event", message).Msg("unable to inspect container")
		return sablier.Message{}, true
	}

	var action sablier.EventAction
	switch message.Action {
	case events.ActionStart, events.ActionUnPause:
		if spec.Config.Healthcheck != nil {
			return sablier.Message{}, true
		}
		action = sablier.EventActionStart
	case events.ActionHealthStatusHealthy:
		action = sablier.EventActionStart
	case events.ActionCreate:
		action = sablier.EventActionCreate
	case events.ActionStop, events.ActionPause:
		action = sablier.EventActionStop
	case events.ActionDestroy:
		action = sablier.EventActionRemove
	default:
		return sablier.Message{}, true
	}

	return sablier.Message{
		Instance: instance,
		Action:   action,
	}, false
}

func (d *DockerProvider) extractInstanceConfigFromEvent(ctx context.Context, message events.Message) (sablier.InstanceConfig, types.ContainerJSON, error) {
	spec, err := d.Client.ContainerInspect(ctx, message.Actor.Attributes["name"])
	if err != nil {
		return sablier.InstanceConfig{}, types.ContainerJSON{}, err
	}
	name := FormatName(spec.Name)
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
	}, spec, nil
}

func (d *DockerProvider) AfterReady(ctx context.Context, name string, considerReadyAfter time.Duration) <-chan error {
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
		log := d.log.With().Str("name", FormatName(c.Name)).Logger()

		action := events.ActionStart
		if d.UsePause && c.State.Paused {
			action = events.ActionUnPause
		} else if c.Config.Healthcheck != nil {
			log.Trace().Msg("container has healthcheck, will be waiting for \"health_status: healthy\"")
			action = events.ActionHealthStatusHealthy
		} else {
			log.Trace().Msg("container has no healthcheck, will be waiting for \"start\"")
		}

		ready := d.AfterAction(ctx, name, action)
		ticker := time.NewTicker(5 * time.Second)

		close(started)
		for {
			select {
			case <-ctx.Done():
				ch <- ctx.Err()
				return
			case <-ticker.C:
				info, spec, err := d.InfoWithSpec(ctx, name)
				if err != nil {
					ch <- ctx.Err()
					return
				}
				if info.Status == sablier.InstanceReady {
					if considerReadyAfter > 0 {
						<-d.considerReadyAfter(spec, considerReadyAfter)
					}
					ch <- nil
					return
				}
			case err = <-ready:
				if err == nil {
					<-time.After(considerReadyAfter)
				}
				ch <- err
				return
			}
		}
	}()
	<-started

	return ch
}

func (d *DockerProvider) AfterAction(ctx context.Context, name string, action events.Action) <-chan error {
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
				d.log.Trace().Any("event", msg).Msg("event received")
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
