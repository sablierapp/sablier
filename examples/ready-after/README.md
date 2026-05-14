# `sablier.ready-after` Example

This example demonstrates the `sablier.ready-after` per-instance label using
[sablierapp/mimic](https://github.com/sablierapp/mimic), a configurable
web-server built for testing purposes.

Some services pass their health check before they can actually serve traffic —
for example a JVM application that opens its HTTP port before loading caches, or
a database that accepts TCP connections before it's ready for queries. The
`sablier.ready-after` label tells Sablier to wait an additional settling period
after the provider reports the instance as started/healthy.

```
Timeline:

  t=0s   Sablier starts the container
  t=5s   mimic becomes running (-running-after=5s)
  t=5s   mimic passes its health check (-healthy=true -healthy-after=5s)
  t=5s   Sablier stamps ReadyAt, begins the 15 s grace period
  t=20s  Grace period elapses → Sablier returns ready to the blocking caller
```

## Services

| Service | Role |
|---|---|
| `sablier` | Manages the `slow-starter` container; exposes the REST API on `:10000` |
| `slow-starter` | `sablierapp/mimic` configured to become healthy after 5 s; carries `sablier.ready-after=15s` |

## Labels on `slow-starter`

```yaml
labels:
  - "sablier.enable=true"
  - "sablier.group=slow-starter"
  - "sablier.ready-after=15s"   # wait 15 s after healthy before unblocking
```

## Prerequisites

- Docker with Compose plugin (`docker compose version`)
- `curl` and `jq` for the walkthrough

## Walkthrough

### 1. Start the stack (slow-starter is stopped initially)

```bash
make up
```

### 2. Watch Sablier logs in a separate terminal

```bash
make logs
```

### 3. Send a blocking request

```bash
make start
```

Sablier will:
1. Ask the provider to start `slow-starter`
2. Poll until the provider reports the container as healthy (`InstanceStatusReady`)
3. Start the 15 s `ReadyAfter` grace period — log lines will show the instance as ready but `IsReady()` returning false
4. After 15 s, return the JSON response to `curl`

### 4. Tear down

```bash
make down
```

## What to look for in the logs

```
level=DEBUG msg="request to check instance status completed" instance=slow-starter new_status=ready
# … 15 s of polling …
level=DEBUG msg="set expiration for instance" instance=slow-starter expiration=2m0s
```

The blocking request holds open until the grace period elapses, then resolves.
