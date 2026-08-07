# Docker Socket Proxy Example

This example shows how to run Sablier **without mounting the Docker socket
directly** into the Sablier container. Instead, a dedicated
[`lscr.io/linuxserver/socket-proxy`](https://github.com/linuxserver/docker-socket-proxy)
container sits between Sablier and the socket, and only the API endpoints that
Sablier actually needs are allowed through.

```
┌─────────────────────────────────────────────────────────────────┐
│ Docker host                                                     │
│                                                                 │
│  ┌─────────┐   DOCKER_HOST=tcp://socket-proxy:2375             │
│  │ sablier │ ──────────────────────────────────────────┐       │
│  └─────────┘                                           ▼       │
│                                           ┌──────────────────┐ │
│                                           │  socket-proxy    │ │
│  ╔═══════════════════════════════════╗    │ (linuxserver)    │ │
│  ║  Allowed endpoints only:          ║    └────────┬─────────┘ │
│  ║  GET  /containers/*               ║             │ unix      │
│  ║  GET  /events                     ║             ▼           │
│  ║  GET  /version                    ║  /var/run/docker.sock   │
│  ║  POST /containers/{id}/start      ║                         │
│  ║  POST /containers/{id}/stop       ║                         │
│  ╚═══════════════════════════════════╝                         │
└─────────────────────────────────────────────────────────────────┘
```

## Why use a socket proxy?

Mounting `/var/run/docker.sock` directly into a container grants that container
**full, unrestricted root-level access** to the Docker daemon. A socket proxy
enforces a least-privilege policy: if Sablier (or a dependency) were ever
compromised, the blast radius is limited to the handful of endpoints the proxy
permits.

## Services

| Service | Image | Role |
|---|---|---|
| `socket-proxy` | `lscr.io/linuxserver/socket-proxy` | Filters Docker API requests; only allows what Sablier needs |
| `sablier` | `sablierapp/sablier` | Connects to Docker via `DOCKER_HOST=tcp://socket-proxy:2375` |
| `whoami` | `acouvreur/whoami` | Managed container — scaled up/down by Sablier |

## Networks

| Network | Internal | Purpose |
|---|---|---|
| `socket-proxy` | yes | Only `socket-proxy` and `sablier` can communicate here; the Docker API is never reachable from the public network |
| `public` | no | Reachable from the host; used by `sablier` and `whoami` |

## Permissions granted to the proxy

| Environment variable | Endpoint(s) | Why Sablier needs it |
|---|---|---|
| `CONTAINERS=1` | `GET /containers/*` | List and inspect managed containers |
| `EVENTS=1` | `GET /events` | Subscribe to the Docker event stream |
| `VERSION=1` | `GET /version` | Initial connection check on startup |
| `POST=1` | `POST /containers/*` | Start, stop, and **wait** for containers — Sablier calls `/containers/{id}/start`, `/containers/{id}/stop`, and `/containers/{id}/wait`; the scope is still bounded by `CONTAINERS=1` |
| `AUTH=0` | `/auth` | Not needed |
| `SECRETS=0` | `/secrets` | Not needed |

> **Why not `ALLOW_START` + `ALLOW_STOP` + `POST=0`?**
> Those two `ALLOW_*` flags bypass the `POST=0` rule only for their specific endpoints.
> Sablier also calls `POST /containers/{id}/wait` after stopping a container to confirm it
> has exited — and there is no `ALLOW_WAIT` flag in the proxy. Setting `POST=1` is therefore
> required. The blast radius is still bounded by `CONTAINERS=1`; all other resource groups
> (`/images`, `/networks`, `/volumes`, etc.) remain blocked.

## Prerequisites

- Docker with Compose plugin (`docker compose version`)

## Walkthrough

### 1. Start everything

```bash
make up
```

### 2. Start the whoami group (blocking strategy)

```bash
make start
```

Sablier will start the `whoami` container and wait for it to be ready, then
return information about the group. You can also open
`http://localhost:10000/api/strategies/blocking?group=whoami&session_duration=1m`
in a browser.

### 3. Check Sablier logs

```bash
make logs
```

### 4. Verify the proxy blocks unauthorised requests

The proxy network is internal, but you can confirm the restriction from within
the Sablier container. Requests to blocked endpoints (e.g. `/images`) will
receive an HTTP `403 Forbidden` response from the proxy.

### 5. Tear down

```bash
make down
```
