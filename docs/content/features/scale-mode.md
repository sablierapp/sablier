---
title: "Scale instead of stop"
description: "Keep an idle workload running at fewer replicas or lower limits."
icon: trending-down
weight: 110
compatibility:
  docker: differs
  swarm: differs
  kubernetes: differs
  podman: differs
  proxmox: impossible
  systemd: differs
  nomad: differs
  ecs: differs
setting: scale-instead-of-stop
aliases:
  - /how-to-guides/scaling-resources/scale-mode/
---

{{< compatibility >}}

Scale a workload down instead of stopping it. When the session expires, Sablier **lowers the replica count** instead of stopping the workload, and **restores it** when a new session is requested.

With an idle replica count of `1` or more, the workload **keeps running** and its resources can be **throttled** while idle (CPU, memory or block I/O, depending on the provider). This removes cold starts, at the cost of keeping the workload alive.

```mermaid
flowchart LR
    active["Active<br/>active replicas and resources"]
    idle["Idle<br/>fewer replicas, throttled CPU and memory"]
    active -->|session expires| idle
    idle -->|new request| active
```

## Idle and active profiles

Every scale-mode setting comes in an `idle` and an `active` variant:

- **`idle.*` is applied when the session expires.**
- **`active.*` is restored when a new session is requested.**

| Setting | Format | Default |
|---------|--------|---------|
| `idle.replicas` | Integer | `0` (stop) |
| `active.replicas` | Integer | `1` |
| `idle.cpu` / `active.cpu` | See [Scale CPU](/features/cpu-limits/) | none |
| `idle.memory` / `active.memory` | See [Scale memory](/features/memory-limits/) | none |
| `idle.blkio-*` / `active.blkio-*` | See [Scale block I/O](/features/disk-io-limits/) | none |

- **`idle.replicas` at `0`**, the default, stops the workload on session expiry and starts it again on demand.
- **`idle.replicas` at `1` or more** keeps the workload running, with optional throttling.
- **Set the `active` value of every limit you set on `idle`.** Depending on the provider, an idle limit is not always cleared on wake-up.

Your provider page lists the profiles it supports, the value formats, and how a change is applied.

## Per-resource guides

{{< cards >}}
  {{< card link="/features/cpu-limits/" icon="chip" title="Scale CPU" subtitle="Throttle CPU when idle, restore on wake-up." >}}
  {{< card link="/features/memory-limits/" icon="server" title="Scale memory" subtitle="Cap memory when idle, restore on wake-up." >}}
  {{< card link="/features/disk-io-limits/" icon="adjustments" title="Scale block I/O" subtitle="Throttle disk bandwidth and IOPS (Docker and systemd)." >}}
{{< /cards >}}
