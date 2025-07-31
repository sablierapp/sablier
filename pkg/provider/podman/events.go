package podman

import (
	"context"

	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/domain/entities/types"

	"log/slog"
	"strings"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	// Set up event filters for container die events
	eventFilters := map[string][]string{
		"type":  {"container"},
		"event": {"died"},
	}

	eventChan := make(chan types.Event)
	cancelChan := make(chan bool)
	err := system.Events(p.conn, eventChan, cancelChan, &system.EventsOptions{
		Filters: eventFilters,
	})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot initialize event stream", slog.Any("error", err))
		close(instance)
		return
	}

	for {
		select {
		case msg, ok := <-eventChan:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				close(instance)
				return
			}
			p.l.DebugContext(ctx, "event stream received", slog.Any("event", msg))
			// Send the container that has died to the channel
			instance <- strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		case <-ctx.Done():
			cancelChan <- true
			close(instance)
			return
		}
	}
}
