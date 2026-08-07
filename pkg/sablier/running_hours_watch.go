package sablier

import (
	"context"
	"log/slog"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
)

// WatchRunningHours keeps configured instances warm during their
// sablier.running-hours window by periodically reconciling all managed
// instances and creating/extending sessions until the window end.
func (s *Sablier) WatchRunningHours(ctx context.Context) {
	ticker := time.NewTicker(s.RunningHoursRefreshFrequency)
	defer ticker.Stop()

	s.reconcileRunningHours(ctx)

	for {
		select {
		case <-ctx.Done():
			s.l.InfoContext(ctx, "stop watching running-hours windows", slog.Any("reason", ctx.Err()))
			return
		case <-ticker.C:
			s.reconcileRunningHours(ctx)
		}
	}
}

func (s *Sablier) reconcileRunningHours(ctx context.Context) {
	instances, err := s.provider.InstanceList(ctx, provider.InstanceListOptions{All: true})
	if err != nil {
		s.l.ErrorContext(ctx, "running-hours reconciliation failed to list instances", slog.Any("error", err))
		return
	}

	now := time.Now()
	for _, configured := range instances {
		if !configured.IsEnabled() {
			continue
		}

		info, err := s.provider.InstanceInspect(ctx, configured.Name)
		if err != nil {
			s.l.WarnContext(ctx, "running-hours reconciliation failed to inspect instance", slog.String("instance", configured.Name), slog.Any("error", err))
			continue
		}
		if info.RunningHours == "" {
			continue
		}

		remaining, inWindow, err := runningHoursRemaining(info.RunningHours, info.RunningDays, now)
		if err != nil {
			s.l.WarnContext(ctx, "invalid running-hours or running-days value, skipping instance", slog.String("instance", info.Name), slog.String("running-hours", info.RunningHours), slog.String("running-days", info.RunningDays), slog.Any("error", err))
			continue
		}
		if !inWindow || remaining <= 0 {
			continue
		}

		_, err = s.InstanceRequest(ctx, info.Name, remaining)
		if err != nil {
			s.l.WarnContext(ctx, "running-hours reconciliation failed to keep instance warm", slog.String("instance", info.Name), slog.Any("error", err))
		}
	}
}
