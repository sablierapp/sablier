# Sablier with OpenTelemetry Example

This example demonstrates how to run Sablier with OpenTelemetry for comprehensive observability.

## What's Included

- **Sablier** - With OpenTelemetry enabled
- **Jaeger** - Distributed tracing backend and UI
- **Prometheus** - Metrics storage
- **Grafana** - Metrics visualization
- **OpenTelemetry Collector** - Optional advanced telemetry pipeline

## Quick Start

### Using Jaeger Directly (Simple Setup)

```bash
# Start the stack
docker-compose up -d sablier jaeger whoami

# Access the UIs
# Jaeger: http://localhost:16686
# Sablier: http://localhost:10000
```

### Using OpenTelemetry Collector (Advanced Setup)

```bash
# Start the full stack
docker-compose up -d

# Access the UIs
# Jaeger: http://localhost:16686
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000
# Sablier: http://localhost:10000
```

## Accessing the UIs

### Jaeger (Tracing)
- URL: http://localhost:16686
- View distributed traces for all Sablier operations
- Search for traces by service, operation, or tags
- Analyze request latency and dependencies

### Prometheus (Metrics)
- URL: http://localhost:9090
- Query raw metrics with PromQL
- Example queries:
  - `sablier_sessions_active` - Active sessions
  - `rate(sablier_instances_started[5m])` - Instance start rate
  - `histogram_quantile(0.95, sablier_requests_duration_bucket)` - 95th percentile request duration

### Grafana (Dashboards)
- URL: http://localhost:3000
- Create custom dashboards
- Connect to Prometheus data source: http://prometheus:9090
- Connect to Jaeger data source: http://jaeger:16686

## Example Queries

### View Traces in Jaeger

1. Open http://localhost:16686
2. Select "sablier" from the Service dropdown
3. Click "Find Traces"
4. Click on any trace to see detailed span information

### Query Metrics in Prometheus

1. Open http://localhost:9090
2. Try these queries:
   ```promql
   # Active sessions by strategy
   sablier_sessions_active
   
   # Total instances started
   sablier_instances_started_total
   
   # Request duration 95th percentile
   histogram_quantile(0.95, rate(sablier_requests_duration_bucket[5m]))
   ```

## Configuration

### Sablier Configuration

The tracing configuration is passed via command-line flags:

```yaml
command:
  - start
  - --provider.name=docker
  - --tracing.enabled=true
  - --tracing.endpoint=jaeger:4317
```

You can also use environment variables:

```yaml
environment:
  - TRACING_ENABLED=true
  - TRACING_ENDPOINT=jaeger:4317
```

### OpenTelemetry Collector Configuration

The collector configuration is in `otel-collector-config.yaml`. It:
- Receives traces and metrics via OTLP (gRPC and HTTP)
- Exports traces to Jaeger
- Exports metrics to Prometheus
- Includes a logging exporter for debugging

### Prometheus Configuration

The Prometheus configuration is in `prometheus-telemetry.yaml`. It scrapes:
- OpenTelemetry Collector metrics endpoint (port 8889)
- Its own metrics endpoint

## Metrics Available

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `sablier_sessions_active` | Gauge | Currently active sessions | `strategy` |
| `sablier_sessions_total` | Counter | Total sessions created | `strategy` |
| `sablier_instances_started` | Counter | Total instances started | `provider` |
| `sablier_instances_stopped` | Counter | Total instances stopped | `provider` |
| `sablier_requests_duration` | Histogram | Request duration in ms | `strategy`, `status` |

## Stopping the Stack

```bash
docker-compose down
```

To remove volumes:

```bash
docker-compose down -v
```

## Troubleshooting

### No traces appearing in Jaeger

1. Check Sablier logs: `docker-compose logs sablier`
2. Verify tracing is enabled in the Sablier startup logs
3. Make sure Jaeger is accessible: `docker-compose logs jaeger`

### No metrics in Prometheus

1. Check if Prometheus can scrape the collector: http://localhost:9090/targets
2. Verify the OpenTelemetry Collector is exporting metrics
3. Check collector logs: `docker-compose -f docker-compose-telemetry.yml logs otel-collector`

### High resource usage

- The full stack with all components can use significant resources
- For lighter setups, use only Jaeger without the collector:
  ```bash
  docker-compose -f docker-compose-telemetry.yml up -d sablier jaeger
  ```

## Learn More

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
