---
title: Distributed Tracing
weight: 84
---

Sablier supports distributed tracing via [OpenTelemetry](https://opentelemetry.io/).
When enabled, every incoming HTTP request and every call made to the underlying
container provider (Docker, Kubernetes, Podman, Proxmox LXC) is captured as a
span and exported to an OTLP-compatible backend such as
[Jaeger](https://www.jaegertracing.io/),
[Grafana Tempo](https://grafana.com/oss/tempo/), or any
[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/).

## What is instrumented

| Component | Mechanism |
|-----------|-----------|
| HTTP server (Gin) | `otelgin` middleware — one span per incoming request |
| Docker provider | `client.WithTraceProvider` — Docker API calls become child spans |
| Docker Swarm provider | same as Docker |
| Podman provider | same as Docker |
| Kubernetes provider | `rest.Config.WrapTransport` — K8s API calls become child spans |
| Proxmox LXC provider | `otelhttp.NewTransport` wrapping the Proxmox HTTP client |
| Webhook dispatcher | `otelhttp.NewTransport` on the outgoing HTTP client |

Trace context is propagated using the **W3C TraceContext** and **Baggage**
formats. If the upstream reverse proxy (Traefik, Nginx, Caddy, …) injects a
`traceparent` header, Sablier will join that trace and all spans will appear
under the same root.

## Configuration

Tracing is **disabled by default**. Enable it in `sablier.yaml` or via
environment variables / CLI flags.

### `sablier.yaml`

```yaml
tracing:
  enabled: true
  exporterType: otlphttp       # otlphttp (default) or stdout
  endpoint: http://localhost:4318   # OTLP collector base URL
  serviceName: sablier          # appears in the tracing UI
  samplingRate: 1.0             # 0.0 = nothing, 1.0 = everything
```

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TRACING_ENABLED` | `false` | Enable tracing |
| `TRACING_EXPORTER_TYPE` | `otlphttp` | `otlphttp` or `stdout` |
| `TRACING_ENDPOINT` | `http://localhost:4318` | OTLP collector base URL |
| `TRACING_SERVICE_NAME` | `sablier` | Service name in the tracing backend |
| `TRACING_SAMPLING_RATE` | `1.0` | Fraction of traces to sample (0.0 – 1.0) |

### CLI flags

```
--tracing.enabled
--tracing.exporter-type   string   (default "otlphttp")
--tracing.endpoint        string   (default "http://localhost:4318")
--tracing.service-name    string   (default "sablier")
--tracing.sampling-rate   float64  (default 1.0)
```

## Exporter types

### `otlphttp` (recommended)

Exports traces using the OTLP/HTTP protocol. Compatible with:

- **Jaeger** ≥ 1.35 (enable with `COLLECTOR_OTLP_ENABLED=true`)
- **Grafana Tempo** (native OTLP ingestion)
- **OpenTelemetry Collector** (configure an `otlp` receiver)
- **Datadog Agent**, **New Relic**, **Honeycomb**, and most modern APM tools

The `endpoint` must be the **base URL of the collector** (scheme + host + port).
Sablier appends `/v1/traces` automatically:

```yaml
tracing:
  enabled: true
  exporterType: otlphttp
  endpoint: http://otel-collector:4318
```

For HTTPS endpoints omit the `http://` scheme (TLS is used by default):

```yaml
tracing:
  enabled: true
  exporterType: otlphttp
  endpoint: https://otel-collector.example.com:4318
```

### `stdout`

Prints spans as human-readable JSON to standard output. Useful for local
development and debugging.

```yaml
tracing:
  enabled: true
  exporterType: stdout
```

## Provider-specific notes

### Docker / Docker Swarm / Podman

The moby SDK client has native OpenTelemetry support. Sablier passes the global
`TracerProvider` via `client.WithTraceProvider`, so every Docker API call
(container start, inspect, events stream, …) becomes a child span of the
request that triggered it.

No additional Docker daemon configuration is required.

### Kubernetes

Sablier wraps the `client-go` HTTP transport via `rest.Config.WrapTransport`.
Every call to the Kubernetes API server (deployments, statefulsets, jobs, …)
appears as a child span. The in-cluster service account token and TLS
configuration are preserved by the wrapping.

### Podman

Same mechanism as Docker. The Podman socket URI is configured via
`provider.podman.uri` (defaults to `unix:///run/podman/podman.sock`).

### Proxmox LXC

The Proxmox Go client's HTTP client is replaced with an `otelhttp`-wrapped
version. If `provider.proxmox-lxc.tls-insecure` is set the original
TLS-insecure transport is wrapped (not bypassed).

## Sampling

Use `samplingRate` to control the volume of traces exported:

| Value | Behaviour |
|-------|-----------|
| `1.0` | Every request is traced (default) |
| `0.5` | 50 % of requests are traced |
| `0.0` | No requests are traced |

For high-traffic deployments set a lower rate and rely on your backend's
tail-based sampling if precise per-request traces are needed.

## Log correlation

When tracing is active, Sablier's structured log lines include `trace_id` and
`span_id` fields so log records can be correlated with spans in your backend:

```
time=2026-01-01T12:00:00Z level=DEBUG msg="Incoming request" ...
  trace_id=4bf92f3577b34da6a3ce929d0e0e4736
  span_id=00f067aa0ba902b7
```

## Example: Docker + Jaeger

See [examples/tracing/jaeger](https://github.com/sablierapp/sablier/tree/main/examples/tracing/jaeger) for a
complete `docker compose` stack with Sablier, a target service, and Jaeger.

```sh
cd examples/tracing/jaeger
make up
make request-blocking   # sends a request and generates traces
make traces             # opens Jaeger UI at http://localhost:16686
```
