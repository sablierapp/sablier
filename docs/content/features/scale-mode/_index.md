---
title: Scale mode
weight: 1
compatibility:
  docker: differs
  swarm: differs
  kubernetes: differs
  podman: differs
  proxmox: impossible
example: scale-mode
---

{{< compatibility >}}

Scale mode is an alternative to stopping containers. Instead of shutting down and restarting the workload, Sablier **scales down the replica count** when the session expires and **restores it** when a new session is requested.

Optionally, when `sablier.idle.replicas >= 1`, the workload **keeps running** at the idle replica count with **throttled resources** (CPU, memory and — on Docker — block I/O), and the resources are **restored** when a new session arrives. This eliminates cold-start latency at the cost of keeping the workload alive.

## Idle and active profiles

Every scale-mode label comes in an `idle` and an `active` variant:

- **`idle.*`** is applied when the session **expires**.
- **`active.*`** is restored when a new session is **requested**.

| Label | Format | Default | Example |
|-------|--------|---------|---------|
| `sablier.idle.replicas` | Integer | `0` (stop) | `"1"` |
| `sablier.active.replicas` | Integer | `1` | `"2"` |
| `sablier.idle.cpu` / `sablier.active.cpu` | see [Scaling CPU](/features/scale-mode/cpu/) | — | `"0.1"`, `"500m"` |
| `sablier.idle.memory` / `sablier.active.memory` | see [Scaling memory](/features/scale-mode/memory/) | — | `"64m"`, `"128Mi"` |
| `sablier.idle.blkio-*` / `sablier.active.blkio-*` | see [Scaling block I/O](/features/scale-mode/block-io/) | — | `"100"`, `"/dev/sda:10m"` |

When `sablier.idle.replicas` is `0` (the default), Sablier stops the workload on session expiry and restarts it on demand. Set it to `1` or higher to keep the workload running with optional resource throttling.

A limit set on the `idle` profile is **not** cleared automatically on wake-up — set the corresponding `active` label to restore it.

## Provider specifics

{{< provider-tabs >}}
{{< provider-tab name="docker" >}}
CPU is decimal cores (`"0.5"` = half a core); memory uses Docker suffixes (`b`, `k`, `m`, `g`). On session expiry Sablier runs the equivalent of `docker update` with the idle limits and restores the active limits on wake-up. The container is never stopped.
{{< /provider-tab >}}
{{< provider-tab name="swarm" >}}
Same value formats as Docker, but the Sablier labels must be placed under `deploy.labels` (not the top-level `labels`) so they attach to the service. Resource changes update the service's task template, triggering a task re-schedule.
{{< /provider-tab >}}
{{< provider-tab name="kubernetes" >}}
CPU and memory use [resource quantities](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-units-in-kubernetes) (`"500m"`, `"2"`, `"128Mi"`, `"1Gi"`).

{{< callout type="info" >}}
Resource limit changes trigger a **rolling restart** of the pods. The service stays available during the transition (old pods are replaced with new ones), with brief overlap.
{{< /callout >}}

{{< callout type="warning" >}}
Scale mode changes resource **limits**, not requests. Ensure your nodes have sufficient allocatable capacity for the active limits.
{{< /callout >}}
{{< /provider-tab >}}
{{< provider-tab name="podman" >}}
Identical to Docker — decimal CPU cores and Docker-style memory units, same labels.
{{< /provider-tab >}}
{{< /provider-tabs >}}

## Per-resource guides

{{< cards >}}
  {{< card link="/features/scale-mode/cpu/" icon="chip" title="Scaling CPU" subtitle="Throttle CPU cores when idle, restore on wake-up." >}}
  {{< card link="/features/scale-mode/memory/" icon="server" title="Scaling memory" subtitle="Cap memory when idle, restore on wake-up." >}}
  {{< card link="/features/scale-mode/block-io/" icon="adjustments" title="Scaling block I/O" subtitle="Throttle disk bandwidth and IOPS (Docker only)." >}}
{{< /cards >}}
