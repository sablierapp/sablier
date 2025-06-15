package systemd

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {

	err := p.Con.Subscribe()
	if err != nil {
		p.l.ErrorContext(ctx, "failed setting up systemd subscription", slog.Any("error", err))
	}

	msgs, errs := p.Con.SubscribeUnits(0)

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				p.l.ErrorContext(ctx, "event stream closed")
				close(instance)
				return
			}
			for name, status := range msg {
				if status == nil || status.ActiveState != "active" {
					instance <- name
				}
			}
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
