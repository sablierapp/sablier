---
title: "Groups"
description: "Start several instances together, and put one instance in several groups."
icon: user-group
weight: 50
compatibility:
  docker: supported
  swarm: supported
  kubernetes: differs
  podman: supported
  proxmox: differs
  systemd: differs
  nomad: supported
  ecs: differs
setting: group
aliases:
  - /features/multiple-groups/
  - /how-to-guides/groups/
---

{{< compatibility >}}

Put an instance in several groups at once by listing them in its `group` setting. A session for **any** of its groups starts it.

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

For example, a `shared-api` instance in the groups `team-a` and `team-b` starts for a `team-a` session and for a `team-b` session.

- **How you write the list depends on your provider.** Your provider page shows the syntax.
- **Spaces around group names are ignored**, and duplicate names are removed.
- **An instance without the setting is in the `default` group.**
- **An instance that loses all its groups**, for example when the setting is removed at runtime, leaves every group it belonged to.
