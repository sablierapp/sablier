package sablier

import (
	"context"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/metrics"
)

// OnInstanceExpired returns a store-expiration callback that stops the
// instance via the provider and records the corresponding metrics.
//
// recorder may be metrics.Noop{} when metrics are disabled — call sites
// must always pass a non-nil recorder.
func OnInstanceExpired(ctx context.Context, provider Provider, recorder metrics.Recorder, logger *slog.Logger) func(string) {
	return onInstanceExpired(ctx, provider, recorder, logger, false)
}

func (s *Sablier) OnInstanceExpired(ctx context.Context) func(string) {
	return onInstanceExpired(ctx, s.provider, s.metrics, s.l, s.ignoreUnlabeled)
}

func onInstanceExpired(ctx context.Context, provider Provider, recorder metrics.Recorder, logger *slog.Logger, ignoreUnlabeled bool) func(string) {
	return func(_key string) {
		go func(key string) {
			logger.InfoContext(ctx, "instance expired", slog.String("instance", key))
			if ignoreUnlabeled {
				info, err := provider.InstanceInspect(ctx, key)
				if err != nil {
					logger.ErrorContext(ctx, "instance expired could not be inspected before stop", slog.String("instance", key), slog.Any("error", err))
					return
				}
				if info.Enabled != "true" {
					logger.WarnContext(ctx, "instance expired but is not managed by sablier, skipping stop", slog.String("instance", key))
					return
				}
			}
			err := provider.InstanceStop(ctx, key)
			if err != nil {
				logger.ErrorContext(ctx, "instance expired could not be stopped from provider", slog.String("instance", key), slog.Any("error", err))
			}
			recorder.RecordInstanceStop(key, "expired")
			recorder.RecordInactiveInstance(key)
			recorder.DiscardReadyWait(key)
		}(_key)
	}
}
