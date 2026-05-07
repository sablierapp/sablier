package docker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	filters := client.Filters{}
	filters.Add("scope", "local")
	filters.Add("type", string(events.ContainerEventType))
	filters.Add("event", "die")
	result := p.Client.Events(ctx, client.EventsListOptions{
		Filters: filters,
	})
	for {
		select {
		case msg, ok := <-result.Messages:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				close(instance)
				return
			}
			// Send the container that has died to the channel
			p.l.DebugContext(ctx, "event received", "event", msg)
			instance <- strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		case err, ok := <-result.Err:
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
