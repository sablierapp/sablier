---
title: Scaling memory
weight: 2
compatibility:
  docker: differs
  swarm: differs
  kubernetes: differs
  podman: differs
  proxmox: impossible
example: scale-mode
---

{{< compatibility >}}

In [scale mode](/features/scale-mode/), Sablier can cap an instance's memory when idle and restore it on wake-up, keeping the workload running instead of stopping it.

## Labels

| Label | Applied when | Format |
|-------|--------------|--------|
| `sablier.idle.memory` | session expires | Docker units (`b`, `k`, `m`, `g`) or Kubernetes quantity |
| `sablier.active.memory` | session requested | same |

Both require `sablier.idle.replicas >= 1`. See [Scale mode](/features/scale-mode/) for the shared model.

## Provider specifics

{{< provider-tabs >}}
{{< provider-tab name="docker" >}}
Memory values use Docker-style suffixes (`b`, `k`, `m`, `g`).

```yaml
services:
  myapp:
    image: myapp:latest
    restart: unless-stopped
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.idle.replicas=1"
      - "sablier.idle.memory=64m"
      - "sablier.active.memory=512m"
```

On session expiry Sablier runs the equivalent of `docker update --memory=64m myapp` and restores `--memory=512m` on wake-up.

{{< callout type="info" >}}
Docker requires the memory swap limit to be updated together with the memory limit. Sablier sets `MemorySwap` equal to `Memory` automatically, which disables swap for the container.
{{< /callout >}}
{{< /provider-tab >}}
{{< provider-tab name="swarm" >}}
Same units as Docker; place the labels under `deploy.labels`.

```yaml
services:
  myapp:
    image: myapp:latest
    deploy:
      replicas: 1
      labels:
        - "sablier.enable=true"
        - "sablier.group=myapp"
        - "sablier.idle.replicas=1"
        - "sablier.idle.memory=64m"
        - "sablier.active.memory=512m"
```
{{< /provider-tab >}}
{{< provider-tab name="kubernetes" >}}
Memory uses the resource-quantity format (`"64Mi"`, `"512Mi"`, `"1Gi"`).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    sablier.enable: "true"
    sablier.group: myapp
    sablier.idle.replicas: "1"
    sablier.idle.memory: "64Mi"
    sablier.active.memory: "512Mi"
```
{{< /provider-tab >}}
{{< provider-tab name="podman" >}}
Identical to Docker — same units and labels.
{{< /provider-tab >}}
{{< /provider-tabs >}}
