package sablier

import (
	"context"
	"log/slog"
)

func OnInstanceExpired(ctx context.Context, provider Provider, logger *slog.Logger) func(string) {
	return func(_key string) {
		go func(key string) {
			logger.InfoContext(ctx, "instance expired", slog.String("instance", key))
			err := provider.InstanceStop(ctx, key)
			if err != nil {
				logger.ErrorContext(ctx, "instance expired could not be stopped from provider", slog.String("instance", key), slog.Any("error", err))
			}
		}(_key)
	}
}
