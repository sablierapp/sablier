---
title: Mark ready on start
description: Treat an instance as ready as soon as it starts, skipping the health check.
weight: 136
aliases:
  - /features/ready-on-start/
compatibility:
  docker: supported
  swarm: supported
  kubernetes: supported
  podman: supported
  proxmox: unsupported
example: ready-on-start
---

{{< compatibility >}}

This guide shows you how to mark an instance ready as soon as it starts, skipping its health check, with the `sablier.ready-on-start` label:

```yaml
# compose.yml
services:
  frigate:
    image: frigate:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=home"
      - "sablier.ready-on-start=true"   # start frigate, don't wait for health
```

Some services only run in the background, such as NVR recorders, cache sidecars, and build agents. The main application works without them being healthy, but you still want Sablier to start them when a request arrives.

Setting `sablier.ready-on-start=true` tells Sablier to dispatch the start but immediately treat the instance as ready, skipping the health check. The reverse proxy passes the request through without waiting.

```mermaid
flowchart LR
    request[Request arrives] --> start[Instance starts]
    start -->|skip health check| ready[Marked ready immediately]
    ready --> pass[Request passes through]
```

- Only the instance with `sablier.ready-on-start=true` is affected. Other instances in the same session still wait for their health checks normally.
- Accepts a Go boolean value such as `"true"` or `"1"`. Invalid values are ignored with a warning. An empty or absent value means no special treatment.
- Works with both dynamic and blocking strategies, with no plugin-side changes needed.
