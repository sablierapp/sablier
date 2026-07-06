---
title: Use a Docker socket proxy
weight: 172
---

Put a dedicated socket proxy between Sablier and `/var/run/docker.sock`, so Sablier reaches only the Docker API endpoints it actually needs.

```yaml
# compose.yml
services:
  socket-proxy:
    image: lscr.io/linuxserver/socket-proxy:latest
    environment:
      - CONTAINERS=1
      - EVENTS=1
      - VERSION=1
      - POST=1
      - AUTH=0
      - SECRETS=0
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - socket-proxy

  sablier:
    image: sablierapp/sablier:1.14.0 # x-release-please-version
    command:
      - start
      - --provider.name=docker
    environment:
      - DOCKER_HOST=tcp://socket-proxy:2375
    depends_on:
      - socket-proxy
    networks:
      - socket-proxy
      - public

  whoami:
    image: acouvreur/whoami:v1.10.2
    labels:
      - "sablier.enable=true"
      - "sablier.group=whoami"
    networks:
      - public

networks:
  socket-proxy:
    internal: true
  public:
```

Sablier reaches the Docker API only through the proxy. Requests to blocked endpoints (for example `/images`) receive an HTTP `403 Forbidden`.

Mounting `/var/run/docker.sock` directly into the Sablier container grants it full, unrestricted root-level access to the Docker daemon. You can instead put a dedicated [socket proxy](https://github.com/linuxserver/docker-socket-proxy) between Sablier and the socket, allowing only the API endpoints Sablier actually needs. If Sablier were ever compromised, the blast radius is limited to that handful of endpoints.

## When to use it

Use this to enforce least-privilege access to the Docker API instead of exposing the raw socket to Sablier.

## How it works

Sablier talks to the proxy over TCP via `DOCKER_HOST=tcp://socket-proxy:2375`; only the proxy mounts the socket, on an internal network unreachable from outside the host. Sablier needs these endpoints:

| Environment variable | Endpoint(s) | Why Sablier needs it |
|---|---|---|
| `CONTAINERS=1` | `GET /containers/*` | List and inspect managed containers |
| `EVENTS=1` | `GET /events` | Subscribe to the Docker event stream |
| `VERSION=1` | `GET /version` | Initial connection check on startup |
| `POST=1` | `POST /containers/*` | Start, stop, and **wait** for containers |

`POST=1` is required because Sablier calls `POST /containers/{id}/wait` after stopping a container, and the proxy has no `ALLOW_WAIT` flag to cover it: the `ALLOW_START` / `ALLOW_STOP` flags are not enough. The scope stays bounded by `CONTAINERS=1`, so all other resource groups (`/images`, `/networks`, `/volumes`, and others) remain blocked.

See the [runnable example](https://github.com/sablierapp/sablier/tree/main/examples/docker-socket-proxy).
