# `auto-stop-externally-started` Example

This example demonstrates the `--provider.auto-stop-externally-started` flag.

Sablier manages containers that carry the `sablier.enable=true` label. If such
a container starts without Sablier having initiated the start (e.g. via
`docker compose up`, a restart policy, or a manual `docker start`), Sablier
will stop it automatically so that traffic is only served once a client
explicitly requests the instance.

Two containers illustrate the difference:

| Service | Labels | Behaviour |
|---------|--------|-----------|
| `managed` | `sablier.enable=true` | Stopped automatically by Sablier |
| `unmanaged` | *(none)* | Left running; Sablier ignores it |

## Flags used

| Flag | Value | Purpose |
|------|-------|---------|
| `--provider.auto-stop-on-startup` | `true` | Stop unregistered managed instances already running at startup |
| `--provider.auto-stop-externally-started` | `true` | Continuously stop managed instances that start while Sablier is running |

## Prerequisites

- Docker with the Compose plugin (`docker compose version`)

## Walkthrough

### Scenario 1 — Startup behaviour

Start the full stack:

```bash
make up
```

Both `managed` and `unmanaged` start via Compose. Because Sablier did not
initiate the `managed` start, `--provider.auto-stop-on-startup` stops it
within seconds of Sablier booting.

Verify:

```bash
make status
```

Expected output (abridged):

```
NAME           STATUS
sablier        Up
managed        Exited
unmanaged      Up
```

### Scenario 2 — Continuous watch

With the stack already running, start `managed` externally:

```bash
make start-managed
```

Sablier receives the Docker `start` event, recognises that it did not initiate
the start, and stops the container again — usually within a second.

Watch it happen in real time:

```bash
make logs
```

You should see a log line similar to:

```
level=INFO msg="externally started instance detected, stopping" instance=managed
```

### Tear down

```bash
make down
```
