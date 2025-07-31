package dockerswarm

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	msgs, errs := p.Client.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "swarm"),
			filters.Arg("type", "service"),
		),
	})

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				p.l.DebugContext(ctx, "event received", "event", msg)
				if msg.Actor.Attributes["replicas.new"] == "0" {
					instance <- msg.Actor.Attributes["name"]
				} else if msg.Action == "remove" {
					instance <- msg.Actor.Attributes["name"]
				}
			case err, ok := <-errs:
				if !ok {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				if errors.Is(err, io.EOF) {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				p.l.ErrorContext(ctx, "event stream error", slog.Any("error", err))
			case <-ctx.Done():
				return
			}
		}
	}()
}
