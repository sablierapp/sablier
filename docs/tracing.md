# Observability with OpenTelemetry

Sablier includes built-in support for OpenTelemetry, providing comprehensive observability through distributed tracing and metrics.

## Configuration

Enable OpenTelemetry by setting the following configuration:

### YAML Configuration

```yaml
tracing:
  enabled: true
  endpoint: localhost:4317  # OTLP gRPC endpoint
```

### Environment Variables

```bash
export TRACING_ENABLED=true
export TRACING_ENDPOINT=localhost:4317
```

### Command-Line Flags

```bash
sablier start --tracing.enabled=true --tracing.endpoint=localhost:4317
```

## Traces

Sablier automatically instruments:

- **HTTP requests** - All incoming requests to the Sablier server
- **Provider operations** - Instance start, stop, inspect, list, and group operations
- **Session management** - Session creation and lifecycle

### Trace Attributes

Each trace includes relevant attributes such as:
- `instance` - Instance name
- `provider` - Provider type (docker, kubernetes, etc.)
- `strategy` - Scaling strategy (dynamic, blocking)
- `http.method` - HTTP method
- `http.route` - HTTP route
- `http.status_code` - HTTP response status code

## Metrics

Sablier exposes the following metrics:

### Counters

- `sablier.sessions.total` - Total number of sessions created
  - Labels: `strategy`
- `sablier.instances.started` - Total number of instances started
  - Labels: `provider`
- `sablier.instances.stopped` - Total number of instances stopped
  - Labels: `provider`

### Gauges

- `sablier.sessions.active` - Number of currently active sessions
  - Labels: `strategy`

### Histograms

- `sablier.requests.duration` - Request duration in milliseconds
  - Labels: `strategy`, `status`

## Using with Jaeger

Example using Jaeger all-in-one:

```bash
# Start Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest

# Start Sablier with tracing
sablier start --tracing.enabled=true --tracing.endpoint=localhost:4317

# View traces at http://localhost:16686
```

## Using with Prometheus + Grafana

Example docker-compose setup:

```yaml
version: '3'
services:
  otel-collector:
    image: otel/opentelemetry-collector:latest
    command: ["--config=/etc/otel-config.yaml"]
    volumes:
      - ./otel-config.yaml:/etc/otel-config.yaml
    ports:
      - "4317:4317"
      - "8889:8889"
  
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yaml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
  
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
  
  sablier:
    image: sablierapp/sablier:latest
    environment:
      - TRACING_ENABLED=true
      - TRACING_ENDPOINT=otel-collector:4317
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

Example OpenTelemetry Collector configuration (`otel-config.yaml`):

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  otlp:
    endpoint: jaeger:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

Example Prometheus configuration (`prometheus.yaml`):

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['otel-collector:8889']
```

## Custom Instrumentation

If you're building custom integrations, you can use the global tracer:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

tracer := otel.Tracer("my-component")
ctx, span := tracer.Start(ctx, "operation-name")
defer span.End()

span.SetAttributes(
    attribute.String("key", "value"),
)

// Your code here
```

## Troubleshooting

### Tracing not working

1. Verify the OpenTelemetry collector is running and accessible
2. Check the endpoint configuration matches your collector
3. Ensure firewall rules allow connections to port 4317
4. Check Sablier logs for tracing initialization errors

### High memory usage

If you experience high memory usage:
- Reduce the batch size in the collector
- Increase the export interval
- Filter traces and metrics at the collector level

### Missing traces

- Ensure `tracing.enabled=true` is set
- Verify the collector is configured to receive OTLP data
- Check that the correct port (4317 for gRPC) is being used
