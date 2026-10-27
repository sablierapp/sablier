---
title: "Ready on start"
description: "Treat an instance as ready as soon as it starts."
icon: play
weight: 80
compatibility:
  docker: supported
  swarm: supported
  kubernetes: supported
  podman: supported
  proxmox: unsupported
  systemd: differs
  nomad: supported
  ecs: supported
setting: ready-on-start
aliases:
  - /how-to-guides/readiness/ready-on-start/
---

{{< compatibility >}}

Start an instance without waiting for it to be ready, with the `ready-on-start` setting. Use it for background services the application works without, such as NVR recorders, cache sidecars or build agents: Sablier starts them on request, and the request passes through immediately.

```mermaid
flowchart LR
    request[Request arrives] --> start[Instance starts]
    start -->|skip readiness check| ready[Marked ready immediately]
    ready --> pass[Request passes through]
```

- **Only that instance skips the wait.** Other instances in the same session still wait until they are ready.
- **The value is a Go boolean**, such as `true` or `1`. An invalid value is ignored with a warning; an empty or absent value changes nothing.
- **It works with both the dynamic and blocking strategies**, with no change to the reverse proxy plugin.
