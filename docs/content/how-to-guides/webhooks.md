---
title: Webhooks
weight: 154
---

Sablier can POST a normalized JSON notification to one or more HTTP endpoints whenever a managed instance **starts** or **stops**.

```yaml
# sablier.yaml
webhooks:
  endpoints:
    - url: https://uptime.example.com/api/push/xxxxxxxx
      headers:
        Authorization: "Bearer <token>"
      events:
        - started
        - stopped
```

Because Sablier sits in front of every supported provider (Docker, Docker Swarm, Kubernetes, Podman, Proxmox LXC), webhooks act as a **unified, provider-agnostic event stream**: your receiver always gets the same payload structure regardless of the underlying runtime.

Common uses:

- Push "up / down" heartbeats to an uptime monitor such as [Uptime Kuma](https://github.com/louislam/uptime-kuma)
- Trigger CI/CD pipelines or custom automation on instance lifecycle events
- Feed a central observability bus

## Configuration

### Fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `url` | Yes | None | Full HTTP(S) URL to POST events to. |
| `headers` | No | `{}` | Key/value map of HTTP request headers (e.g. `Authorization`). |
| `events` | No | all | Event types that trigger this endpoint. Accepted values: `started`, `stopped` (lifecycle) and `activate`, `deactivate` (intent). Omit or leave empty to receive **all lifecycle** events. |

Multiple endpoints are supported. Each endpoint is evaluated independently so you can send different event types to different targets.

### Event types

| Event | Kind | Fired when |
|-------|------|-----------|
| `started` | Lifecycle | Sablier observes a managed instance scale up. |
| `stopped` | Lifecycle | Sablier observes a managed instance scale to zero. |
| `activate` | Intent | Sablier wants a [delegated-scaling](/how-to-guides/scaling-resources/delegated-scaling/) instance brought up (an external scaler owns the replica count). |
| `deactivate` | Intent | Sablier wants a delegated-scaling instance idled. |

**Lifecycle** events (`started`/`stopped`) are observations of an actual scale transition. An endpoint with an empty `events` filter receives all of them.

**Intent** events (`activate`/`deactivate`) are emitted only for workloads labelled `sablier.delegate-scaling=true`. They require an **explicit subscription**: an endpoint receives them **only** if its `events` filter lists them — there is no empty-means-all fallback, so existing unfiltered endpoints see zero intent traffic. See [Delegated scaling](/how-to-guides/scaling-resources/delegated-scaling/).

### CLI / environment variables

Webhook endpoints can also be set via command-line flags or environment variables using Viper's standard dotted-path mapping:

| YAML key | CLI flag | Environment variable |
|----------|----------|---------------------|
| `webhooks.endpoints[0].url` | `--webhooks.endpoints[0].url` | `SABLIER_WEBHOOKS_ENDPOINTS_0_URL` |

For multiple endpoints it is easiest to use the YAML configuration file.

## Payload format

Every delivery is an HTTP `POST` with `Content-Type: application/json`. The body is a single JSON object:

```json
{
  "event": "started",
  "instance": {
    "name": "my-nginx"
  },
  "timestamp": "2025-01-15T10:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event` | `string` | Normalized event type: `"started"` or `"stopped"` (lifecycle), or `"activate"` / `"deactivate"` (intent). |
| `instance.name` | `string` | Name of the container / service / workload as known to Sablier. |
| `timestamp` | RFC 3339 UTC | Time at which Sablier processed the event. |

The payload is intentionally minimal and **provider-agnostic**. The same structure is emitted whether the instance is a Docker container, a Kubernetes Deployment, a Docker Swarm service, or any other supported provider. Future fields may be added in a backwards-compatible manner (new fields only).

## Delivery semantics

Delivery guarantees differ by event kind.

**Lifecycle events** (`started`/`stopped`) are best-effort:

- Delivered **asynchronously** (fire-and-forget). They do not block the event loop.
- Each endpoint is called in its own goroutine.
- HTTP errors (status 400 or above) and network errors are **logged** but do **not** affect Sablier's behavior.
- The HTTP client uses a **10-second timeout** per request.
- There is **no retry**. If a delivery fails, the event is lost. Use an intermediary queue (e.g., a message broker) in front of your endpoint if at-least-once delivery is required.

**Intent events** (`activate`/`deactivate`) are delivered with stronger guarantees, because a reordered or dropped delivery would leave the external scaler in the wrong state:

- Delivered **strictly in order**, one event at a time — the receiver observes them in the exact order Sablier emitted them.
- Each delivery is retried up to **5 attempts** with a linear backoff before Sablier gives up and moves on.
- The same 10-second per-request timeout applies.

See [Delegated scaling](/how-to-guides/scaling-resources/delegated-scaling/) for the full picture.

## Example: Uptime Kuma integration

[Uptime Kuma](https://github.com/louislam/uptime-kuma) supports push-style heartbeat monitoring. Configure a "Push" monitor in Uptime Kuma and paste its heartbeat URL as a Sablier webhook endpoint:

```yaml
webhooks:
  endpoints:
    - url: https://uptime.example.com/api/push/<monitor-id>?status=up&msg=started
      events:
        - started
    - url: https://uptime.example.com/api/push/<monitor-id>?status=down&msg=stopped
      events:
        - stopped
```

> **Note**: Uptime Kuma heartbeat URLs accept query parameters. Since Sablier always sends a POST body, you may need to configure Uptime Kuma to interpret the HTTP request (not just the body) when routing status. Check the Uptime Kuma documentation for the exact URL format.

## Example: filter by event type

Only notify when an instance stops (useful for alerting):

```yaml
webhooks:
  endpoints:
    - url: https://alerts.example.com/hooks/instance-stopped
      events:
        - stopped
```

Only notify when an instance starts:

```yaml
webhooks:
  endpoints:
    - url: https://analytics.example.com/hooks/instance-started
      events:
        - started
```
