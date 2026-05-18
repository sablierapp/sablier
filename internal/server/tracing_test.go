package server

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
)

// TestOtelGin_SpanCreatedPerRequest verifies that the otelgin middleware
// creates one span per HTTP request when a real TracerProvider is installed.
func TestOtelGin_SpanCreatedPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set up an in-memory span exporter so we can inspect recorded spans.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		// Restore the default noop provider so other tests are unaffected.
		otel.SetTracerProvider(otel.GetTracerProvider())
	})

	strategy := &api.ServeStrategy{
		Metrics: metrics.Noop{},
	}
	tracingCfg := config.Tracing{ServiceName: "sablier-test"}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, tracingCfg, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	spans := exporter.GetSpans()
	assert.NotEmpty(t, spans, "at least one span should be recorded per HTTP request")
}

// TestOtelGin_NoopWhenTracingDisabled verifies that no real spans are emitted
// when the global provider is the default noop provider.
func TestOtelGin_NoopWhenTracingDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use an in-memory exporter on a SEPARATE provider that is NOT installed
	// globally, so the router uses the noop global provider.
	exporter := tracetest.NewInMemoryExporter()
	_ = sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	// global provider intentionally left as noop

	strategy := &api.ServeStrategy{
		Metrics: metrics.Noop{},
	}
	tracingCfg := config.Tracing{ServiceName: "sablier-test"}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, tracingCfg, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	assert.Empty(t, exporter.GetSpans(), "noop global provider should not record spans")
}
