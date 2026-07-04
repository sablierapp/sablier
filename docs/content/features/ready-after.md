---
title: Ready after
weight: 4
compatibility:
  docker: supported
  swarm: supported
  kubernetes: supported
  podman: supported
  proxmox: unsupported
example: ready-after
---

{{< compatibility >}}

Some services are started (or pass their health check) before they finish initialising — for example a JVM application that opens its HTTP port before loading all caches, a database that accepts TCP connections before it's ready for queries, or any container without a health check that needs a few extra seconds after start-up before it can serve traffic.

Setting `sablier.ready-after` introduces a mandatory settling delay. Once the provider reports the instance as ready — whether that means it has started or passed its health check — Sablier continues to return a *not-ready* response to any blocking or dynamic request until the grace period elapses.

```yaml
services:
  myapp:
    image: myapp:latest
    restart: unless-stopped
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.ready-after=30s"  # wait 30 s after started/healthy before unblocking requests
```

The value is a Go duration string. Valid examples:

| Value | Duration |
|-------|----------|
| `500ms` | 500 milliseconds |
| `30s` | 30 seconds |
| `1m30s` | 1 minute 30 seconds |
| `2m` | 2 minutes |

If the label is absent or set to an unparseable value, no extra wait is applied.

{{< callout type="warning" >}}
The `sablier.ready-after` grace period counts from when the instance **first** becomes ready in a given session. It does not reset on subsequent requests.
{{< /callout >}}
