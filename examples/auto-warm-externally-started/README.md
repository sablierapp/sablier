# `auto-warm-externally-started` Example

This example demonstrates the `--provider.auto-warm-externally-started` flag.

Sablier manages containers that carry the `sablier.enable=true` label. If such
a container starts without Sablier having initiated the start (e.g. via
`docker compose up`, a deployment tool, or a manual `docker start`), Sablier
seeds a session for it with the default session duration instead of stopping
it. The container keeps running until that session expires without traffic,
then hibernates through the regular scale-to-zero lifecycle.

This is the non-destructive counterpart to
[`auto-stop-externally-started`](../auto-stop-externally-started): fresh
deployments are adopted into the session lifecycle instead of being killed.
The two flags are mutually exclusive; Sablier refuses to start with both
enabled.

Two containers illustrate the difference:

| Service | Labels | Behaviour |
|---------|--------|-----------|
| `managed` | `sablier.enable=true` | Receives a seeded session; stopped only when it expires |
| `unmanaged` | *(none)* | Left running; Sablier ignores it |

## Flags used

| Flag | Value | Purpose |
|------|-------|---------|
| `--provider.auto-stop-on-startup` | `false` | Do not stop already-running managed instances at startup |
| `--provider.auto-warm-externally-started` | `true` | Continuously seed a session for managed instances that were started externally |
| `--sessions.default-duration` | `1m` | Duration of the seeded session (shortened for the demo) |

## Prerequisites

- Docker with the Compose plugin (`docker compose version`)

## Walkthrough

### Scenario 1 — Adoption instead of destruction

Start the full stack:

```bash
make up
```

Both `managed` and `unmanaged` start via Compose. Because Sablier did not
initiate the `managed` start, `--provider.auto-warm-externally-started` seeds
a 1m session for it — the container is **not** stopped.

Watch it happen:

```bash
make logs
```

You should see a log line similar to:

```
level=INFO msg="seeded session for externally started instance" instance=managed duration=1m0s
```

### Scenario 2 — Hibernation once the session expires

Do not send any traffic to `managed`. About a minute later the seeded session
expires and Sablier stops the container:

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

### Scenario 3 — Continuous watch

Start `managed` externally again:

```bash
make start-managed
```

Sablier receives the Docker `start` event, recognises that it did not initiate
the start, and seeds a fresh session — the cycle repeats.

### Tear down

```bash
make down
```
