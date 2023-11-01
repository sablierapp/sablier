package docker

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/acouvreur/sablier/internal/provider"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

func (client *Client) Events(ctx context.Context) (<-chan provider.Message, <-chan error) {
	msgs, errs := client.Client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "local"),
			filters.Arg("type", events.ContainerEventType),
		),
	})

	messages := make(chan provider.Message)
	closed := make(chan error, 1)

	started := make(chan struct{})
	go func() {
		defer close(closed)

		close(started)
		for {
			select {
			case msg := <-msgs:
				event, ignore := client.parseEvent(ctx, msg)

				// Some events are ignored
				if !ignore {
					messages <- event
				}
			case err := <-errs:
				if errors.Is(err, io.EOF) {
					closed <- err
					return
				}
			}
		}
	}()
	<-started

	return messages, closed
}

func (client *Client) SubscribeOnce(ctx context.Context, name string, action provider.EventAction, wait chan<- error) {
	msgs, errs := client.Client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "local"),
			filters.Arg("type", events.ContainerEventType),
		),
	})

	go func() {
		for {
			select {
			case <-ctx.Done():
				ctx.Err()
			case msg := <-msgs:
				event, ignore := client.parseEvent(ctx, msg)
				// Some events are ignored
				if !ignore {
					if event.Name == name && event.Action == action {
						wait <- nil
						return
					}
				}
			case err := <-errs:
				if errors.Is(err, io.EOF) {
					wait <- err
					return
				}
			}
		}
	}()
}

func (client *Client) parseEvent(ctx context.Context, msg events.Message) (provider.Message, bool) {
	name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")

	// When a 'start' action is incoming, we must check wether the container has a healthcheck,
	// so we can wait for it to be healthy
	switch msg.Action {

	case "start":
		container, err := client.Client.ContainerInspect(ctx, name)
		if err != nil {
			// Issue inspecting the container, should panic?
			return provider.Message{}, true
		}
		return provider.Message{
			Name:   name,
			Action: provider.EventActionStart,
		}, container.State.Health != nil

	case "die":
		return provider.Message{
			Name:   name,
			Action: provider.EventActionStop,
		}, false

	case "destroy":
		return provider.Message{
			Name:   name,
			Action: provider.EventActionDestroy,
		}, false
	case "create":
		return provider.Message{
			Name:   name,
			Action: provider.EventActionCreate,
		}, false

	default:
		healthStatus := strings.Split(msg.Action, ": ")
		if len(healthStatus) == 2 && healthStatus[1] == "healthy" {
			return provider.Message{
				Name:   name,
				Action: provider.EventActionStart,
			}, false
		} else {
			return provider.Message{}, true
		}
	}
}
