# `sablier.ready-on-start` Example

This example demonstrates the `sablier.ready-on-start` per-instance label using
[sablierapp/mimic](https://github.com/sablierapp/mimic), a configurable
web-server built for testing purposes.

Some services don't need to be healthy for the application to function —
background processes, cache sidecars, video recorders, etc. With
`sablier.ready-on-start=true`, Sablier dispatches the start but immediately
reports the instance as ready, so the reverse proxy passes the request through
without waiting.

```
Timeline:

  t=0s   Request arrives, Sablier dispatches container start
  t=0s   Sablier returns X-Sablier-Session-Status: ready   ← no wait
  t=0s   Proxy passes user request through to backend
  t=5s   Container becomes healthy (in the background)
```

## Services

| Service | Role |
|---|---|
| `sablier` | Manages containers; exposes REST API on `:10000` |
| `web` | `sablierapp/mimic` configured to become healthy after 5 s |
| `nvr` | `sablierapp/mimic` with `sablier.ready-on-start=true` — background recorder that doesn't block responses |

## Labels on `nvr`

```yaml
labels:
  - "sablier.enable=true"
  - "sablier.group=home"
  - "sablier.ready-on-start=true"   # don't wait for health
```

## Prerequisites

- Docker with Compose plugin (`docker compose version`)
- `curl` and `jq` for the walkthrough

## Walkthrough

### 1. Start the stack (containers are stopped initially)

```bash
make up
```

### 2. Send a blocking request for the group

```bash
make start
```

Sablier will:
1. Dispatch the start for both `web` and `nvr`
2. `nvr` has `ready-on-start=true` → immediately considered ready, no wait added
3. `web` blocks until its health check passes (~10 s)
4. The session returns after all instances are ready (~10 s instead of ~20 s without `ready-on-start`)

The `nvr` instance never delays the session even though it starts from cold.

### 3. Tear down

```bash
make down
```

## What to look for in the logs

```
level=DEBUG msg="request to start instance dispatched" instance=nvr status=starting
# nvr is immediately reported as ready despite status=starting
level=DEBUG msg="set expiration for instance" instance=nvr expiration=1m0s
# web still needs its health check
level=INFO msg="instance is ready" instance=web
```
