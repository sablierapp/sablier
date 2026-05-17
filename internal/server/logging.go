package server

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

// StructuredLogger logs a gin HTTP request in JSON format. Allows to set the
// logger for testing purposes.
func StructuredLogger(logger *slog.Logger) gin.HandlerFunc {
	return sloggin.NewWithConfig(logger, sloggin.Config{
		DefaultLevel:     slog.LevelDebug,
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,

		WithUserAgent:      false,
		WithRequestID:      true,
		WithRequestBody:    false,
		WithRequestHeader:  false,
		WithResponseBody:   false,
		WithResponseHeader: false,
		// Emit trace/span IDs when OpenTelemetry tracing is active so log
		// lines can be correlated with traces in the observability backend.
		// Both fields are zero-valued (and omitted) when tracing is disabled.
		WithSpanID:  true,
		WithTraceID: true,

		Filters: []sloggin.Filter{},
	})
}
