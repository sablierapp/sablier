package sablier

import (
	"context"
	"errors"
	"log/slog"
)

func OnInstanceExpired(ctx context.Context, provider Provider, logger *slog.Logger) func(string) {
	return func(_key string) {
		go func(key string) {
			logger.InfoContext(ctx, "instance expired", slog.String("instance", key))
			err := provider.InstanceStop(ctx, key)
			if err != nil {
				if errors.As(err, &ErrInstanceNotManaged{}) {
					logger.WarnContext(ctx, "instance expired but is not managed by sablier, skipping stop", slog.String("instance", key))
					return
				}
				logger.ErrorContext(ctx, "instance expired could not be stopped from provider", slog.String("instance", key), slog.Any("error", err))
			}
		}(_key)
	}
}
