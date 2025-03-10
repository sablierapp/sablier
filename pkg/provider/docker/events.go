package docker

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"io"
	"log/slog"
	"strings"
)

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
