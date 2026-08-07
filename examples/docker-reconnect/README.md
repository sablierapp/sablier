# Docker Socket Proxy — Reconnect Example

This example demonstrates sablier's **event stream reconnect** behaviour when the
Docker API becomes temporarily unavailable.

Instead of mounting `/var/run/docker.sock` directly into sablier, traffic is
routed through a thin TCP proxy (`socat`).  Stopping that proxy cuts the
connection; restarting it lets sablier's built-in reconnect loop recover
automatically — no restart of sablier required.

```
┌─────────────────────────────────────────────────────┐
│ Docker host                                         │
│                                                     │
│  ┌─────────┐   DOCKER_HOST=tcp://docker-proxy:2375  │
│  │ sablier │ ──────────────────────────────────┐    │
│  └─────────┘                                   ▼    │
│                                    ┌──────────────┐ │
│                                    │ docker-proxy │ │
│                                    │   (socat)    │ │
│                                    └──────┬───────┘ │
│                                           │ unix    │
│                                           ▼         │
│                               /var/run/docker.sock  │
└─────────────────────────────────────────────────────┘
```

## How it works

| Service | Image | Role |
|---|---|---|
| `docker-proxy` | `alpine/socat` | Forwards `TCP :2375` → `/var/run/docker.sock` |
| `sablier` | `sablierapp/sablier` | Connects to Docker via `DOCKER_HOST=tcp://docker-proxy:2375` |
| `whoami` | `acouvreur/whoami` | Managed container used to trigger events |

## Prerequisites

- Docker with Compose plugin (`docker compose version`)

## Walkthrough

### 1. Start everything

```bash
make up
```

### 2. Watch sablier logs in a separate terminal

```bash
make logs
```

### 3. Disconnect the proxy (simulate a Docker API outage)

```bash
make disconnect
```

Sablier will log a series of `reconnecting event stream` warnings with
linear back-off (1 s, 2 s, … capped at 30 s).

### 4. Restore the proxy

```bash
make reconnect
```

Once the proxy is back, sablier re-establishes the event stream and resumes
normal operation — you will see the reconnect log line disappear and normal
event processing continue.

### 5. Tear down

```bash
make down
```

## What to look for in the logs

```
level=WARN  msg="reconnecting event stream" attempt=1 backoff=1s
level=WARN  msg="reconnecting event stream" attempt=2 backoff=2s
...
level=DEBUG msg="event received" event=...   ← back to normal after reconnect
```

The reconnect loop retries up to **10 times** before giving up, so a brief
outage (a few tens of seconds) will be recovered transparently.
