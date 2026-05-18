# `webhooks` Example

This example demonstrates Sablier's webhook feature. Sablier posts a
normalized JSON event to every configured endpoint whenever a managed
instance **starts** or **stops** — regardless of the underlying provider
(Docker, Kubernetes, Swarm, Podman, …).

## Services

| Service | Purpose |
|---------|---------|
| `sablier` | Sablier server with webhook dispatcher configured |
| `whoami` | Managed container (`sablier.enable=true`, group `demo`) |
| `webhook-receiver` | [`mendhak/http-https-echo`](https://github.com/mendhak/http-https-echo) — echoes every POST it receives |

## Prerequisites

- Docker with the Compose plugin (`docker compose version`)
- `curl` and `jq` (optional, for the `make start` shortcut)

## Walkthrough

### 1 — Start the stack

```bash
make up
```

This starts Sablier, the `whoami` target, and the `webhook-receiver` echo
server. The `whoami` container will be in a stopped state (Sablier manages it).

### 2 — Trigger a session (fires a "started" webhook)

```bash
make start
```

This calls the Sablier blocking strategy for the `demo` group. Sablier starts
`whoami` and immediately POSTs to `http://webhook-receiver:8080/webhook`.

### 3 — Inspect the webhook payload

```bash
make receiver-logs
```

You will see the raw HTTP request logged by the echo server. The payload looks
like:

```json
{
  "event": "started",
  "instance": {
    "name": "webhooks-whoami-1"
  },
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### 4 — Wait for expiry (fires a "stopped" webhook)

After the 1-minute session expires, Sablier stops `whoami` and fires a second
webhook:

```json
{
  "event": "stopped",
  "instance": {
    "name": "webhooks-whoami-1"
  },
  "timestamp": "2025-01-15T10:31:00Z"
}
```

Watch Sablier logs with:

```bash
make logs
```

### 5 — Tear down

```bash
make down
```

## Configuration explained

The Sablier configuration is embedded in `compose.yml` via a Docker config:

```yaml
webhooks:
  endpoints:
    - url: http://webhook-receiver:8080/webhook
      # No "events" filter → receives both "started" and "stopped"
```

To restrict to a single event type, add an `events` filter:

```yaml
webhooks:
  endpoints:
    - url: http://webhook-receiver:8080/webhook
      events:
        - stopped  # only fires when an instance stops
```

Custom HTTP headers (e.g. for authentication) can be added:

```yaml
webhooks:
  endpoints:
    - url: https://uptime.example.com/api/push/xxxxxxxx
      headers:
        Authorization: "Bearer <token>"
```
