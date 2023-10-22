package swarm

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
			filters.Arg("type", "service"),
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
				event, ignore := client.parseEvent(ctx, msg, messages)

				// Some event are ignored
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

// On create events, register
func (client *Client) parseEvent(ctx context.Context, msg events.Message, messages <-chan provider.Message) (provider.Message, bool) {
	name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")

	switch msg.Action {

	case "create":
		// TODO: Start service status polling
		return provider.Message{}, true

	case "update":
		if msg.Actor.Attributes["replicas.new"] == "0" {
			// TODO: Stop service status polling
			return provider.Message{
				Name:   name,
				Action: provider.EventActionStop,
			}, false
		} else if msg.Actor.Attributes["replicas.old"] == "0" {
			// TODO: Start service status polling
			return provider.Message{}, true
		} else {
			return provider.Message{}, true
		}

	case "remove":
		// TODO: Stop service status polling
		return provider.Message{
			Name:   name,
			Action: provider.EventActionStop,
		}, false

	default:
		return provider.Message{}, true
	}
}
