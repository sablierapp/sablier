---
title: Mark instances as optional (best-effort)
description: Start an instance with the session without letting its readiness or failures block the session.
weight: 137
compatibility:
  docker: supported
  swarm: supported
  kubernetes: supported
  podman: supported
  proxmox: supported
---

{{< compatibility >}}

This guide shows you how to mark an instance as **optional** (best-effort) by prefixing its name with `optional:` in a `names` request:

```yaml
# Traefik middleware example
http:
  middlewares:
    my-sablier:
      plugin:
        sablier:
          sablierUrl: http://sablier:10000
          names: whoami,optional:whoami-helper
          dynamic:
            displayName: My Stack
```

An optional instance behaves exactly like any other session member — it is started on demand, its session is refreshed by traffic, and it scales back down when the session expires — with one difference: **its readiness and errors never gate the session**. The waiting page clears (and a blocking request returns) as soon as all non-optional instances are ready, even if an optional instance is still starting, unhealthy, or failed to start entirely.

Use this to split a stack into important instances that visitors must wait for and auxiliary ones (workers, reporting, developer tooling) that should come up alongside them but must never block the stack when they are broken.

## How it works

The `optional:` marker travels inside the instance name, so every reverse proxy plugin forwards it unchanged — no plugin upgrade is needed. Sablier strips the prefix when the session is requested; providers, sessions and metrics all see the clean instance name. No workload name can collide with the marker because Docker, Swarm, Podman and Kubernetes names cannot contain a colon.

The prefix works the same in a direct API call:

```text
GET /api/strategies/dynamic?names=whoami&names=optional:whoami-helper&session_duration=5m
```

## Notes

- A session whose members are **all** optional is immediately ready.
- Optional instances still appear on the waiting page details while it is shown, but they are not listed as a blocking reason in timeout errors.
- Optionality is per request: the same instance can be required in one middleware and optional in another.
- For group-based sessions, instances are discovered via labels and cannot be marked optional yet; use `names` for split behaviour.

## Related

- [Mark ready on start](/how-to-guides/readiness/ready-on-start/) — skip the health check but still require a successful start.
- [Settling delay](/how-to-guides/readiness/ready-after/) — the opposite direction: require extra readiness time.
