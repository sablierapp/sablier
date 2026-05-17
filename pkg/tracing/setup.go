// Package tracing configures the global OpenTelemetry TracerProvider based on
// the application configuration and exposes helpers for instrumenting HTTP
// clients.
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/version"
)

// ShutdownFunc flushes and stops the TracerProvider gracefully.
type ShutdownFunc func(context.Context) error

// Setup initialises an OpenTelemetry TracerProvider based on cfg, installs
// it as the global provider together with W3C TraceContext + Baggage
// propagators, and returns a ShutdownFunc that must be called before the
// process exits so all in-flight spans are flushed.
//
// logger is used to redirect gRPC-internal logs (channel state changes,
// resolver updates, etc.) through the application's slog handler instead of
// writing them directly to stderr. gRPC "Info" events are demoted to
// slog.LevelDebug so they remain invisible at the default log level.
//
// When cfg.Enabled is false, Setup is a no-op and the returned ShutdownFunc
// does nothing.
func Setup(ctx context.Context, cfg config.Tracing, logger *slog.Logger) (ShutdownFunc, error) {
	noop := func(context.Context) error { return nil }

	if !cfg.Enabled {
		return noop, nil
	}

	// Redirect gRPC internal logs through slog before any gRPC connection is
	// attempted by the exporter.
	setGRPCLogger(logger)

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tracing: build resource: %w", err)
	}

	exporter, err := buildExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tracing: build exporter: %w", err)
	}

	sampler := buildSampler(cfg.SamplingRate)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func buildResource(ctx context.Context, cfg config.Tracing) (*resource.Resource, error) {
	svcVersion := version.Version
	if svcVersion == "" {
		svcVersion = "dev"
	}
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(svcVersion),
		),
		// Use individual process detectors instead of resource.WithProcess()
		// to avoid the process-owner lookup that requires cgo or $USER.
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithOS(),
		resource.WithHost(),
	)
}

func buildExporter(ctx context.Context, cfg config.Tracing) (sdktrace.SpanExporter, error) {
	switch cfg.ExporterType {
	case "otlphttp", "":
		return buildOTLPHTTPExporter(ctx, cfg.Endpoint)
	case "stdout":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		return nil, fmt.Errorf("unknown exporter type %q: supported values are otlphttp, stdout", cfg.ExporterType)
	}
}

// buildOTLPHTTPExporter parses the endpoint URL and creates the OTLP HTTP
// exporter. The endpoint may include a scheme ("http://" or "https://"); if
// the scheme is "http" the exporter is configured without TLS.
func buildOTLPHTTPExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("tracing: invalid endpoint %q: %w", endpoint, err)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(parsed.Host),
	}
	if parsed.Scheme == "http" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	return otlptracehttp.New(ctx, opts...)
}

func buildSampler(rate float64) sdktrace.Sampler {
	if rate <= 0 {
		return sdktrace.NeverSample()
	}
	if rate >= 1 {
		return sdktrace.AlwaysSample()
	}
	return sdktrace.TraceIDRatioBased(rate)
}
