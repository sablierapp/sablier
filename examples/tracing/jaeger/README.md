# OpenTelemetry tracing example — Docker provider + Jaeger

A minimal Docker Compose stack showing Sablier exporting distributed traces via
OTLP to a Jaeger all-in-one backend. Every incoming API request and every Docker
API call made by the provider are visible as spans in the Jaeger UI.

## What it runs

| Service    | Image                          | Purpose                                         |
|------------|--------------------------------|-------------------------------------------------|
| `sablier`  | `sablierapp/sablier`           | Sablier with tracing enabled, sends to Jaeger   |
| `whoami`   | `acouvreur/whoami:v1.10.2`     | Target service in group `demo`                  |
| `jaeger`   | `jaegertracing/all-in-one`     | Receives OTLP HTTP on :4318, serves UI on :16686|

Sablier is configured with:

```yaml
tracing:
  enabled: true
  exporterType: otlphttp
  endpoint: http://jaeger:4318
  serviceName: sablier
  samplingRate: 1.0
```

## Prerequisites

- Docker and `docker compose` v2

## Running

```sh
make up                # start the stack (waits for Jaeger to be healthy)
make request-blocking  # trigger a blocking request to wake the "demo" group
make traces            # open Jaeger UI (macOS) at http://localhost:16686
```

In the Jaeger UI:

1. Select **Service → sablier**.
2. Click **Find Traces**.
3. Expand a trace to see:
   - The root span for the incoming HTTP request (`GET /api/strategies/blocking`).
   - Child spans for Docker API calls (`container start`, `container inspect`, etc.).

## Suggested demo loop

1. `make stop-target` — stop `whoami` to force a cold start.
2. `make request-blocking` — Sablier starts `whoami` and blocks until ready.
3. Open Jaeger UI → search for the `sablier` service → inspect the trace.
4. Observe that Docker API calls (`POST /containers/{id}/start`,
   `GET /containers/{id}/json`) appear as child spans under the incoming
   HTTP request span.
5. Wait one minute for the session to expire, then repeat.

## Tearing down

```sh
make down
```

## Notes

- Traces are stored in Jaeger's in-memory store and are lost on restart.
- Sampling rate is set to `1.0` (all requests sampled). Reduce it via
  `tracing.samplingRate` for high-traffic deployments.
- The `otelgin` middleware automatically propagates W3C `traceparent` headers,
  so if a reverse proxy or client sends trace context Sablier will join that
  trace instead of starting a new root span.
