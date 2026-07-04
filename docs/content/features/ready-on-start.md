---
title: Ready on start
weight: 5
compatibility:
  docker: supported
  swarm: supported
  kubernetes: supported
  podman: supported
  proxmox: unsupported
example: ready-on-start
---

{{< compatibility >}}

Some services only run in the background — NVR recorders, cache sidecars, build agents, etc. The main application works without them being healthy, but you still want Sablier to start them when a request arrives.

Setting `sablier.ready-on-start=true` tells Sablier to dispatch the start but immediately treat the instance as ready, skipping the health check. The reverse proxy passes the request through without waiting.

```yaml
services:
  frigate:
    image: frigate:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=home"
      - "sablier.ready-on-start=true"   # start frigate, don't wait for health
```

- Only the instance with `sablier.ready-on-start=true` is affected. Other instances in the same session still wait for their health checks normally.
- Accepts a Go boolean value (`"true"`, `"1"`, …). Invalid values are ignored with a warning. An empty or absent value means no special treatment.
- Works with both dynamic and blocking strategies — no plugin-side changes needed.
