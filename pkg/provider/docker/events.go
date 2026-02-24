package docker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/provider"
)

func (p *Provider) Events(ctx context.Context) (<-chan provider.Event, <-chan error) {
	msgs := make(chan provider.Event)
	errs := make(chan error)

	started := make(chan struct{})
	go func() {
		defer close(errs)

		dockerMsgs, dockerErrs := p.Client.Events(ctx, events.ListOptions{
			Filters: filters.NewArgs(
				filters.Arg("scope", "local"),
				filters.Arg("type", string(events.ContainerEventType)),
				filters.Arg("event", string(events.ActionCreate)),
				filters.Arg("event", string(events.ActionStart)),
				filters.Arg("event", string(events.ActionHealthStatusHealthy)),
				filters.Arg("event", string(events.ActionHealthStatusUnhealthy)),
				filters.Arg("event", string(events.ActionStop)),
				filters.Arg("event", string(events.ActionDie)),
				filters.Arg("event", string(events.ActionDestroy)),
			),
		})

		close(started)
		for {
			select {
			case err, ok := <-dockerErrs:
				if !ok {
					p.l.ErrorContext(ctx, "docker event stream closed")
					errs <- errors.New("docker event stream closed")
					return
				}
				if errors.Is(err, io.EOF) {
					p.l.ErrorContext(ctx, "docker event stream closed")
					errs <- errors.New("docker event stream closed")
					return
				}
				p.l.ErrorContext(ctx, "docker event stream error", slog.Any("error", err))
				errs <- err
				return

			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case msg, ok := <-dockerMsgs:
				if !ok {
					errs <- errors.New("docker event stream closed")
					return
				}

				p.l.DebugContext(ctx, "event received", "event", msg)
				if shouldIgnoreEvent(msg) {
					p.l.DebugContext(ctx, "ignoring event", "event", msg)
					continue
				}

				// Send the event to the channel
				msgs <- p.dockerEventToProviderEvent(ctx, msg)
			}
		}
	}()
	<-started

	return msgs, errs
}

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	msgs, errs := p.Client.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "local"),
			filters.Arg("type", string(events.ContainerEventType)),
			filters.Arg("event", "die"),
		),
	})
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				close(instance)
				return
			}
			// Send the container that has died to the channel
			p.l.DebugContext(ctx, "event received", "event", msg)
			instance <- strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		case err, ok := <-errs:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				close(instance)
				return
			}
			if errors.Is(err, io.EOF) {
				p.l.ErrorContext(ctx, "event stream closed")
				close(instance)
				return
			}
			p.l.ErrorContext(ctx, "event stream error", slog.Any("error", err))
		case <-ctx.Done():
			close(instance)
			return
		}
	}
}

func shouldIgnoreEvent(msg events.Message) bool {
	enabled, ok := msg.Actor.Attributes["sablier.enable"]

	// If the label is not set, we ignore the event. This means that only containers that are explicitly enabled will be managed by Sablier.
	if !ok {
		return true
	}

	// If the label is set to false, we ignore the event. This allows users to explicitly exclude containers from being managed by Sablier.
	if enabled == "false" {
		return true
	}

	// In all other cases, we do not ignore the event.
	return false
}

func (p *Provider) dockerEventToProviderEvent(ctx context.Context, msg events.Message) provider.Event {
	name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	group, ok := msg.Actor.Attributes["sablier.group"]
	if !ok {
		group = "default"
	}

	action := p.dockerEventActionToProviderEventAction(ctx, msg)

	return provider.Event{
		ProviderName:  "docker",
		Type:          provider.EventTypeInstance,
		Action:        action,
		InstanceName:  name,
		InstanceGroup: group,
		Event:         msg,
	}
}

func (p *Provider) dockerEventActionToProviderEventAction(ctx context.Context, msg events.Message) provider.EventAction {
	action := events.Action(msg.Action)
	switch action {
	case events.ActionCreate:
		return provider.EventActionCreated

	case events.ActionStart, events.ActionUnPause, events.ActionRestart:
		inspected, err := p.Client.ContainerInspect(ctx, msg.Actor.ID)
		if err != nil {
			return provider.EventActionPending
		}
		if inspected.Config.Healthcheck == nil {
			return provider.EventActionRunning
		}
		return provider.EventActionPending

	case events.ActionHealthStatusHealthy:
		return provider.EventActionRunning

	case events.ActionHealthStatusUnhealthy:
		return provider.EventActionFailed

	case events.ActionStop, events.ActionKill:
		return provider.EventActionStopping

	case events.ActionPause, events.ActionDie, events.ActionOOM:
		return provider.EventActionStopped

	case events.ActionDestroy:
		return provider.EventActionRemoved

	default:
		return provider.EventActionUnknown
	}
}
