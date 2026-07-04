---
title: Multiple groups
weight: 6
compatibility:
  docker: supported
  swarm: supported
  kubernetes: differs
  podman: supported
  proxmox: differs
example: multiple-groups
---

{{< compatibility >}}

An instance can belong to more than one group at once. When a session is requested for **any** of its groups, the instance is started.

Practical rules:

- Spaces around separators are ignored (`"team-a , team-b"` is equivalent to `"team-a,team-b"`).
- Duplicate group names are deduplicated silently.
- An instance that loses all its group membership (e.g. the label/tag removed at runtime) is dropped from every group it belonged to.

## Provider specifics

{{< provider-tabs >}}
{{< provider-tab name="docker" >}}
Provide a **comma-separated** list in the `sablier.group` label:

```yaml
services:
  shared-api:
    image: myorg/shared-api:latest
    restart: unless-stopped
    labels:
      - "sablier.enable=true"
      - "sablier.group=team-a,team-b"   # member of both groups
```

A session for `team-a` starts every instance whose groups include `team-a` — including `shared-api` — and the same instance also starts for a `team-b` session.
{{< /provider-tab >}}
{{< provider-tab name="swarm" >}}
Same comma-separated `sablier.group` label as Docker, under `deploy.labels`:

```yaml
services:
  shared-api:
    image: myorg/shared-api:latest
    deploy:
      labels:
        - "sablier.enable=true"
        - "sablier.group=team-a,team-b"
```
{{< /provider-tab >}}
{{< provider-tab name="kubernetes" >}}
A comma-separated value is **not** valid as a Kubernetes label value, so set `sablier.group` as an **annotation**. Keep `sablier.enable` as a label so workload discovery still finds it:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shared-api
  labels:
    sablier.enable: "true"
  annotations:
    sablier.group: "team-a,team-b"
```
{{< /provider-tab >}}
{{< provider-tab name="podman" >}}
Identical to Docker — a comma-separated `sablier.group` label.
{{< /provider-tab >}}
{{< provider-tab name="proxmox" >}}
Proxmox LXC uses **tags** instead of key-value labels. Add one `sablier-group-<name>` tag per group:

- `sablier-group-team-a`
- `sablier-group-team-b`

The instance then belongs to both `team-a` and `team-b`.
{{< /provider-tab >}}
{{< /provider-tabs >}}
