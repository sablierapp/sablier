package server

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

// StructuredLogger logs a gin HTTP request in JSON format. Allows to set the
// logger for testing purposes.
func StructuredLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger.Enabled(context.TODO(), slog.LevelDebug) {
		return sloggin.NewWithConfig(logger, sloggin.Config{
			DefaultLevel:     slog.LevelInfo,
			ClientErrorLevel: slog.LevelWarn,
			ServerErrorLevel: slog.LevelError,

			WithUserAgent:      false,
			WithRequestID:      true,
			WithRequestBody:    false,
			WithRequestHeader:  false,
			WithResponseBody:   false,
			WithResponseHeader: false,
			WithSpanID:         false,
			WithTraceID:        false,

			Filters: []sloggin.Filter{},
		})
	}

	return sloggin.NewWithConfig(logger, sloggin.Config{
		DefaultLevel:     slog.LevelInfo,
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,

		WithUserAgent:      false,
		WithRequestID:      true,
		WithRequestBody:    false,
		WithRequestHeader:  false,
		WithResponseBody:   false,
		WithResponseHeader: false,
		WithSpanID:         false,
		WithTraceID:        false,

		Filters: []sloggin.Filter{},
	})
}
