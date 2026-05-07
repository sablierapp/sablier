package dockerswarm

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/moby/moby/client"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	filters := client.Filters{}
	filters.Add("scope", "swarm")
	filters.Add("type", "service")
	result := p.Client.Events(ctx, client.EventsListOptions{
		Filters: filters,
	})

	go func() {
		for {
			select {
			case msg, ok := <-result.Messages:
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
			case err, ok := <-result.Err:
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
