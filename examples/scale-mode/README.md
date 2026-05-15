# Scale Mode Example

This example demonstrates Sablier's **scale mode**: instead of stopping containers when sessions expire, Sablier scales down the replica count (and optionally throttles CPU/memory). When a new session is requested, replicas and resources are restored.

## How It Works

Scale mode is controlled by two pairs of labels: `sablier.idle.*` (applied on session expiry) and `sablier.active.*` (applied when a session is requested).

The **replica count** is the primary control:

| `sablier.idle.replicas` | Behaviour on session expiry |
|-------------------------|-----------------------------|
| `0` (default) | Workload is **stopped** (same as without scale mode) |
| `1` or more | Workload **keeps running** at the given replica count |

CPU and memory throttling only activates when `sablier.idle.replicas >= 1`. If you only set `sablier.idle.cpu` or `sablier.idle.memory` without also setting `sablier.idle.replicas=1`, no throttling is applied.

### This example (throttle, don't stop)

1. **Session expires** → replicas stay at 1, CPU is throttled to 0.1 core and memory to 64 MB.
2. **New request arrives** → replicas stay at 1, CPU is restored to 2.0 cores and memory to 512 MB. Container is immediately ready — no restart needed.

### Scaling to more than 1 replica

You can set `sablier.active.replicas` to any value your infrastructure supports. For example, to run 3 active replicas on Docker Swarm:

```yaml
deploy:
  labels:
    - "sablier.enable=true"
    - "sablier.group=myapp"
    - "sablier.idle.replicas=1"    # keep 1 replica alive when idle
    - "sablier.idle.cpu=0.1"
    - "sablier.idle.memory=64m"
    - "sablier.active.replicas=3"  # scale up to 3 when a session is requested
    - "sablier.active.cpu=2.0"
    - "sablier.active.memory=512m"
```

Or simply scale replicas without any CPU/memory throttling:

```yaml
labels:
  - "sablier.enable=true"
  - "sablier.group=myapp"
  - "sablier.idle.replicas=1"   # keep running at 1 replica when idle
  - "sablier.active.replicas=3" # scale up to 3 on demand
```

## Running the Example

```bash
# Start everything
make up

# Trigger an active session and watch the resource transition
make demo

# Tear down
make down
```

## Manual Steps

```bash
docker compose up -d

# Request a session (blocks until whoami is ready, then returns)
curl 'http://localhost:10000/api/strategies/blocking?group=whoami&timeout=30s'

# Inspect current CPU/memory limits (should be active: ~2 000 000 000 nanocores)
docker inspect scale-mode-whoami-1 --format '{{.HostConfig.NanoCPUs}} nanocores'

# Wait for the 30 s session to expire, then check again (should be idle: ~100 000 000 nanocores)
sleep 35
docker inspect scale-mode-whoami-1 --format '{{.HostConfig.NanoCPUs}} nanocores'
```

## Labels Used

| Label | Value | Meaning |
|-------|-------|---------|
| `sablier.idle.replicas` | `1` | Keep 1 replica alive when idle (required to enable resource throttling) |
| `sablier.idle.cpu` | `0.1` | Throttle to 10% of one CPU when idle |
| `sablier.idle.memory` | `64m` | Throttle to 64 MB when idle |
| `sablier.active.replicas` | `1` | Replica count to restore when a session is requested |
| `sablier.active.cpu` | `2.0` | Restore to 2 full CPUs when active |
| `sablier.active.memory` | `512m` | Restore to 512 MB when active |

## Breaking Change from Previous Versions

Before this release, adding `sablier.idle.cpu` or `sablier.idle.memory` labels was enough to enable resource throttling. **You must now also set `sablier.idle.replicas=1`** (or higher) to opt into resource throttling mode.

If `sablier.idle.replicas` is absent or `0`, Sablier stops the workload normally regardless of any CPU/memory labels.

**Migration:** add `sablier.idle.replicas=1` to any workload that previously used `sablier.idle.cpu` / `sablier.idle.memory`.

## Notes

- Scale mode is supported for **Docker**, **Docker Swarm**, **Kubernetes** (Deployments and StatefulSets), and **Podman**.
- For Kubernetes, replica and resource changes trigger a rolling restart. The service stays available during the transition.
- CPU/memory labels are optional. You can use replicas alone (no throttling).

### Memory swap (Docker)

Docker requires the memory swap limit to be updated in the same `docker update` call as the memory limit. Sablier handles this automatically by setting `MemorySwap` equal to `Memory`, which disables swap for the container.
