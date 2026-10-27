---
title: "Disk I/O limits"
description: "Throttle an instance's disk I/O while it is idle."
icon: database
weight: 140
compatibility:
  docker: supported
  swarm: unsupported
  kubernetes: unsupported
  podman: unsupported
  proxmox: impossible
  systemd: supported
  nomad: impossible
  ecs: impossible
setting: block-io
aliases:
  - /features/scale-mode/block-io/
  - /how-to-guides/scaling-resources/scale-blkio/
---

{{< compatibility >}}

Throttle an instance's block I/O when it is idle, with the `idle.blkio-*` and `active.blkio-*` settings. Use it when an idle workload should keep running but must not compete with active workloads for disk bandwidth.

```mermaid
flowchart LR
    active["Active<br/>blkio-weight 500"]
    idle["Idle<br/>blkio-weight 100"]
    active -->|session expires| idle
    idle -->|new request| active
```

| Setting (idle / active) | Format | Example |
|-------------------------|--------|---------|
| `idle.blkio-weight` / `active.blkio-weight` | Integer `10`–`1000` | `100`, `500` |
| `idle.blkio-weight-device` / `active.blkio-weight-device` | `path:weight` list | `/dev/sda:100` |
| `idle.blkio-device-read-bps` / `active.blkio-device-read-bps` | `path:rate` list | `/dev/sda:10m` |
| `idle.blkio-device-write-bps` / `active.blkio-device-write-bps` | `path:rate` list | `/dev/sda:10m` |
| `idle.blkio-device-read-iops` / `active.blkio-device-read-iops` | `path:iops` list | `/dev/sda:100` |
| `idle.blkio-device-write-iops` / `active.blkio-device-write-iops` | `path:iops` list | `/dev/sda:100` |

- **`blkio-weight` is a relative scheduling weight from `10` to `1000`.** Values outside that range, or `0`, are ignored.
- **Rates use byte units in powers of 1024:** `10m` is 10 MiB/s, `100k` is 100 KiB/s.
- **IOPS values are plain integers.**
- **Per-device values are `path:value` pairs**, separated by commas.
- **All of them need `idle.replicas` at `1` or more**, and you should set the `active` value of every limit you set on `idle`. See [Scale instead of stop](/features/scale-mode/).

Your provider page shows how the limits are applied, and their requirements.
