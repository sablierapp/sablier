---
title: Instance Groups
description: Make one instance belong to several groups at once.
weight: 138
aliases:
  - /features/multiple-groups/
compatibility:
  docker: supported
  swarm: supported
  kubernetes: differs
  podman: supported
  proxmox: differs
example: multiple-groups
---

{{< compatibility >}}

This guide shows you how to make an instance belong to more than one group at once by providing a **comma-separated** list in the `sablier.group` label:

```yaml
# compose.yml
services:
  shared-api:
    image: myorg/shared-api:latest
    restart: unless-stopped
    labels:
      - "sablier.enable=true"
      - "sablier.group=team-a,team-b"   # member of both groups
```

When a session is requested for **any** of its groups, the instance is started. A session for `team-a` starts every instance whose groups include `team-a`, including `shared-api`, and the same instance also starts for a `team-b` session.

```mermaid
flowchart LR
    reqA["Request for team-a"]
    reqB["Request for team-b"]
    ga["Group team-a"]
    gb["Group team-b"]
    api["shared-api instance"]
    reqA --> ga
    reqB --> gb
    ga -->|start| api
    gb -->|start| api
```

Practical rules:

- Spaces around separators are ignored (`"team-a , team-b"` is equivalent to `"team-a,team-b"`).
- Duplicate group names are deduplicated silently.
- An instance that loses all its group membership (e.g. the label/tag removed at runtime) is dropped from every group it belonged to.
