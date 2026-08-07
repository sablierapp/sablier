---
title: Scale block I/O
description: Throttle a workload's block I/O while idle (Docker only).
weight: 124
aliases:
  - /features/scale-mode/block-io/
compatibility:
  docker: supported
  swarm: unsupported
  kubernetes: unsupported
  podman: unsupported
  proxmox: impossible
example: scale-mode
---

{{< compatibility >}}

This guide shows you how to throttle an instance's **block I/O** when idle on the **Docker** provider, using the `sablier.idle.blkio-*` and `sablier.active.blkio-*` labels:

```yaml
# compose.yml
services:
  myapp:
    image: myapp:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.idle.replicas=1"
      # Global relative weight (10-1000)
      - "sablier.idle.blkio-weight=100"
      - "sablier.active.blkio-weight=500"
      # Per-device limits (comma-separate multiple devices: "/dev/sda:10m,/dev/sdb:5m")
      - "sablier.idle.blkio-device-read-bps=/dev/sda:10m"
      - "sablier.idle.blkio-device-write-bps=/dev/sda:10m"
      - "sablier.idle.blkio-device-read-iops=/dev/sda:100"
      - "sablier.idle.blkio-device-write-iops=/dev/sda:100"
```

In addition to CPU and memory, Docker can throttle block I/O in [scale mode](/how-to-guides/scaling-resources/scale-mode/). This is useful when an idle workload should keep running but must not compete with active workloads for disk bandwidth.

```mermaid
flowchart LR
    active["Active<br/>blkio-weight 500"]
    idle["Idle<br/>blkio-weight 100"]
    active -->|session expires| idle
    idle -->|new request| active
```

{{< callout type="info" >}}
Block I/O throttling is currently **Docker only**. Docker Swarm, Kubernetes and Podman ignore these labels.
{{< /callout >}}

## Labels

| Label (idle / active) | Format | Example |
|------------------------|--------|---------|
| `sablier.idle.blkio-weight` / `sablier.active.blkio-weight` | integer `10`-`1000` | `"100"`, `"500"` |
| `sablier.idle.blkio-weight-device` / `...active...` | `path:weight` list | `"/dev/sda:100"` |
| `sablier.idle.blkio-device-read-bps` / `...active...` | `path:rate` list (Docker units) | `"/dev/sda:10m"` |
| `sablier.idle.blkio-device-write-bps` / `...active...` | `path:rate` list | `"/dev/sda:10m"` |
| `sablier.idle.blkio-device-read-iops` / `...active...` | `path:iops` list | `"/dev/sda:100"` |
| `sablier.idle.blkio-device-write-iops` / `...active...` | `path:iops` list | `"/dev/sda:100"` |

- `blkio-weight` is a relative I/O scheduling weight in the range `10`-`1000`. Values outside that range (or the value `0`) are ignored.
- `*-bps` rates accept Docker-style byte units (`10m` = 10 MB/s, `100k` = 100 KB/s).
- `*-iops` rates are plain integers.
- Per-device values are `path:value` pairs; separate multiple devices with commas.
- As with CPU/memory, a limit set on the idle profile is **not** cleared automatically on wake-up. To restore full I/O, set the corresponding `sablier.active.*` label (e.g. a higher `blkio-weight` or a larger rate).

See [Applying labels](/reference/labels/#applying-labels) for how each provider expresses labels.

{{< callout type="warning" >}}
**Per-device blkio limits require a Docker daemon with API version 1.55 or newer.** Older daemons accept the update request but silently ignore the per-device fields (`blkio-weight-device`, `blkio-device-read-bps`, `blkio-device-write-bps`, `blkio-device-read-iops`, `blkio-device-write-iops`). The container's cgroup is left unchanged. See [moby/moby#52650](https://github.com/moby/moby/issues/52650). Sablier logs a warning when it detects this situation. The global `blkio-weight` label is unaffected and works on all supported Docker versions.
{{< /callout >}}
