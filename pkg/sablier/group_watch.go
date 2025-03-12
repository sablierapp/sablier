package sablier

import (
	"context"
	"log/slog"
	"time"
)

func (s *Sablier) GroupWatch(ctx context.Context) {
	// This should be changed to event based instead of polling.
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ctx.Done():
			s.l.InfoContext(ctx, "stop watching groups", slog.Any("reason", ctx.Err()))
			return
		case <-ticker.C:
			groups, err := s.provider.InstanceGroups(ctx)
			if err != nil {
				s.l.ErrorContext(ctx, "cannot retrieve group from provider", slog.Any("reason", err))
			} else if groups != nil {
				s.SetGroups(groups)
			}
		}
	}
}
