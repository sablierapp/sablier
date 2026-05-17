package tracing_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/tracing"
)

func TestSetup_DisabledIsNoop(t *testing.T) {
	cfg := config.Tracing{Enabled: false}

	shutdown, err := tracing.Setup(context.Background(), cfg, slog.Default())

	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// When disabled, the global provider should remain the default no-op provider.
	tp := otel.GetTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	assert.False(t, span.SpanContext().IsValid(), "noop span should have an invalid (zero) SpanContext")
	span.End()

	require.NoError(t, shutdown(context.Background()))
}

func TestSetup_StdoutExporter(t *testing.T) {
	cfg := config.Tracing{
		Enabled:      true,
		ExporterType: "stdout",
		ServiceName:  "sablier-test",
		SamplingRate: 1.0,
	}

	shutdown, err := tracing.Setup(context.Background(), cfg, slogt.New(t))

	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Verify a real tracer provider is installed.
	tp := otel.GetTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	assert.True(t, span.SpanContext().IsValid(), "span from SDK provider should have a valid SpanContext")
	span.End()

	require.NoError(t, shutdown(context.Background()))

	// Restore the global noop provider so subsequent tests are unaffected.
	otel.SetTracerProvider(otel.GetTracerProvider())
}

func TestSetup_UnknownExporterReturnsError(t *testing.T) {
	cfg := config.Tracing{
		Enabled:      true,
		ExporterType: "grpc-nonsense",
		ServiceName:  "sablier-test",
		SamplingRate: 1.0,
	}

	shutdown, err := tracing.Setup(context.Background(), cfg, slogt.New(t))

	assert.Error(t, err)
	assert.Nil(t, shutdown)
}

func TestSetup_OTLPHTTPExporterBadEndpoint(t *testing.T) {
	cfg := config.Tracing{
		Enabled:      true,
		ExporterType: "otlphttp",
		Endpoint:     "://bad-url",
		ServiceName:  "sablier-test",
		SamplingRate: 1.0,
	}

	shutdown, err := tracing.Setup(context.Background(), cfg, slogt.New(t))

	assert.Error(t, err)
	assert.Nil(t, shutdown)
}

func TestSetup_SamplerAlwaysSample(t *testing.T) {
	cfg := config.Tracing{
		Enabled:      true,
		ExporterType: "stdout",
		ServiceName:  "sablier-test",
		SamplingRate: 1.0,
	}

	shutdown, err := tracing.Setup(context.Background(), cfg, slogt.New(t))
	require.NoError(t, err)
	defer shutdown(context.Background()) //nolint:errcheck

	_, span := otel.Tracer("test").Start(context.Background(), "always-sampled")
	assert.True(t, span.SpanContext().IsSampled())
	span.End()
}

func TestSetup_SamplerNeverSample(t *testing.T) {
	cfg := config.Tracing{
		Enabled:      true,
		ExporterType: "stdout",
		ServiceName:  "sablier-test",
		SamplingRate: 0.0,
	}

	shutdown, err := tracing.Setup(context.Background(), cfg, slogt.New(t))
	require.NoError(t, err)
	defer shutdown(context.Background()) //nolint:errcheck

	_, span := otel.Tracer("test").Start(context.Background(), "never-sampled")
	assert.False(t, span.SpanContext().IsSampled())
	span.End()
}

func TestTracingConfig_Defaults(t *testing.T) {
	cfg := config.NewTracingConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "otlphttp", cfg.ExporterType)
	assert.Equal(t, "http://localhost:4318", cfg.Endpoint)
	assert.Equal(t, "sablier", cfg.ServiceName)
	assert.Equal(t, 1.0, cfg.SamplingRate)
}
