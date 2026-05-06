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
	return func(_key string) {
		go func(key string) {
			logger.InfoContext(ctx, "instance expired", slog.String("instance", key))
			err := provider.InstanceStop(ctx, key)
			if err != nil {
				logger.ErrorContext(ctx, "instance expired could not be stopped from provider", slog.String("instance", key), slog.Any("error", err))
			}
			recorder.RecordInstanceStop(key, "expired")
			recorder.RecordInactiveInstance(key)
		}(_key)
	}
}
