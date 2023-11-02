package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"os"
	"time"
)

func InitLog() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   false,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	})
	ctxHandler := ContextHandler{jsonHandler}
	return slog.New(ctxHandler)
}

func GinSlogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		ctx := c.Request.Context()
		end := time.Now().UTC()
		latency := end.Sub(start)
		fields := slog.Group("http",
			slog.Int("status", c.Writer.Status()),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.String("ip", c.ClientIP()),
			slog.String("user-agent", c.Request.UserAgent()),
			slog.Duration("latency", latency),
		)
		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logger.ErrorContext(ctx, e, fields)
			}
		} else {
			logger.InfoContext(ctx, path, fields)
		}
	}
}

type ContextHandler struct {
	slog.Handler
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(h.addTraceFromContext(ctx)...)
	return h.Handler.Handle(ctx, r)
}

func (h ContextHandler) addTraceFromContext(ctx context.Context) (as []slog.Attr) {
	if ctx == nil {
		return
	}
	span := trace.SpanContextFromContext(ctx)
	traceID := span.TraceID().String()
	spanID := span.SpanID().String()
	traceGroup := slog.Group("trace", slog.String("id", traceID))
	spanGroup := slog.Group("span", slog.String("id", spanID))
	as = append(as, traceGroup)
	as = append(as, spanGroup)
	return
}
