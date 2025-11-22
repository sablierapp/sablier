package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string
	Enabled        bool
}

type Telemetry struct {
	tracerProvider *trace.TracerProvider
	meterProvider  *metric.MeterProvider
	logger         *slog.Logger
}

// New initializes OpenTelemetry with tracing and metrics
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Telemetry, error) {
	if !cfg.Enabled {
		logger.Info("OpenTelemetry disabled")
		return &Telemetry{logger: logger}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Setup Tracer Provider
	traceExporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// Setup Meter Provider
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(10*time.Second))),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// Setup propagators
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry initialized",
		"service", cfg.ServiceName,
		"version", cfg.ServiceVersion,
		"endpoint", cfg.Endpoint)

	return &Telemetry{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		logger:         logger,
	}, nil
}

// Shutdown gracefully shuts down the telemetry providers
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.tracerProvider == nil && t.meterProvider == nil {
		return nil
	}

	var err error
	if t.tracerProvider != nil {
		if shutdownErr := t.tracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = shutdownErr
			t.logger.Error("failed to shutdown tracer provider", "error", err)
		}
	}

	if t.meterProvider != nil {
		if shutdownErr := t.meterProvider.Shutdown(ctx); shutdownErr != nil {
			err = shutdownErr
			t.logger.Error("failed to shutdown meter provider", "error", err)
		}
	}

	return err
}
