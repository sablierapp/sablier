package tracing

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// OTelHandler wraps any slog.Handler and injects the active OTel trace-ID and
// span-ID into every log record whose context carries a valid span.
//
// When no span is active — because tracing is disabled or the log call was
// made outside a traced request — the handler is a transparent pass-through
// with zero allocation overhead beyond a single context value lookup.
//
// Usage: wrap the inner handler before passing it to slog.New:
//
//	logger := slog.New(tracing.NewOTelHandler(tint.NewHandler(os.Stderr, opts)))
type OTelHandler struct {
	inner slog.Handler
}

// NewOTelHandler returns an OTelHandler that enriches records from inner with
// trace_id and span_id attributes when an active OTel span is present.
func NewOTelHandler(inner slog.Handler) *OTelHandler {
	return &OTelHandler{inner: inner}
}

func (h *OTelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *OTelHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *OTelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &OTelHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *OTelHandler) WithGroup(name string) slog.Handler {
	return &OTelHandler{inner: h.inner.WithGroup(name)}
}
