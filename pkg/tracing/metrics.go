package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	sessionsActive   metric.Int64UpDownCounter
	sessionsTotal    metric.Int64Counter
	instancesStarted metric.Int64Counter
	instancesStopped metric.Int64Counter
	requestsDuration metric.Float64Histogram
}

func InitMetrics() (*Metrics, error) {
	meter := otel.Meter("sablier")

	sessionsActive, err := meter.Int64UpDownCounter("sablier.sessions.active",
		metric.WithDescription("Number of currently active sessions"))
	if err != nil {
		return nil, err
	}

	sessionsTotal, err := meter.Int64Counter("sablier.sessions.total",
		metric.WithDescription("Total number of sessions created"))
	if err != nil {
		return nil, err
	}

	instancesStarted, err := meter.Int64Counter("sablier.instances.started",
		metric.WithDescription("Total number of instances started"))
	if err != nil {
		return nil, err
	}

	instancesStopped, err := meter.Int64Counter("sablier.instances.stopped",
		metric.WithDescription("Total number of instances stopped"))
	if err != nil {
		return nil, err
	}

	requestsDuration, err := meter.Float64Histogram("sablier.requests.duration",
		metric.WithDescription("Duration of requests in milliseconds"),
		metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}

	return &Metrics{
		sessionsActive:   sessionsActive,
		sessionsTotal:    sessionsTotal,
		instancesStarted: instancesStarted,
		instancesStopped: instancesStopped,
		requestsDuration: requestsDuration,
	}, nil
}

func (m *Metrics) RecordSessionStart(ctx context.Context, strategy string) {
	if m == nil {
		return
	}
	m.sessionsActive.Add(ctx, 1, metric.WithAttributes(
		attribute.String("strategy", strategy),
	))
	m.sessionsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("strategy", strategy),
	))
}

func (m *Metrics) RecordSessionEnd(ctx context.Context, strategy string) {
	if m == nil {
		return
	}
	m.sessionsActive.Add(ctx, -1, metric.WithAttributes(
		attribute.String("strategy", strategy),
	))
}

func (m *Metrics) RecordInstanceStart(ctx context.Context, provider string) {
	if m == nil {
		return
	}
	m.instancesStarted.Add(ctx, 1, metric.WithAttributes(
		attribute.String("provider", provider),
	))
}

func (m *Metrics) RecordInstanceStop(ctx context.Context, provider string) {
	if m == nil {
		return
	}
	m.instancesStopped.Add(ctx, 1, metric.WithAttributes(
		attribute.String("provider", provider),
	))
}

func (m *Metrics) RecordRequestDuration(ctx context.Context, duration float64, strategy string, status string) {
	if m == nil {
		return
	}
	m.requestsDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("strategy", strategy),
		attribute.String("status", status),
	))
}
