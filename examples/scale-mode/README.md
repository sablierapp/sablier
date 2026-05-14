# Scale Mode Example

This example demonstrates Sablier's **scale mode**: instead of stopping containers when sessions expire, Sablier throttles their CPU and memory. When a new session is requested, resources are restored and the container is immediately available — no cold-start wait.

## How It Works

1. **Session expires** → Sablier runs `docker update --cpus=0.1 --memory=64m whoami`. Container stays running, using minimal resources.
2. **New request arrives** → Sablier runs `docker update --cpus=2.0 --memory=512m whoami`. Resources are restored. The container is immediately ready (no restart needed).

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
curl 'http://localhost:10000/api/strategies/blocking?names=whoami&timeout=30s'

# Inspect current CPU/memory limits (should be active: ~2 000 000 000 nanocores)
docker inspect scale-mode-whoami-1 --format '{{.HostConfig.NanoCPUs}} nanocores'

# Wait for the 30 s session to expire, then check again (should be idle: ~100 000 000 nanocores)
sleep 35
docker inspect scale-mode-whoami-1 --format '{{.HostConfig.NanoCPUs}} nanocores'
```

## Labels Used

| Label | Value | Meaning |
|-------|-------|---------|
| `sablier.idle.cpu` | `0.1` | 10% of one CPU when idle |
| `sablier.idle.memory` | `64m` | 64 MB when idle |
| `sablier.active.cpu` | `2.0` | 2 full CPUs when active |
| `sablier.active.memory` | `512m` | 512 MB when active |

## Notes

- Scale mode is supported for **Docker**, **Docker Swarm**, **Kubernetes**, and **Podman**.
- For Kubernetes, a resource limit change triggers a rolling restart. The service stays available during the transition.
- You can set only some labels (e.g. only `sablier.idle.cpu` without a memory limit). Labels not set default to 0 (unlimited).

### Memory swap (Docker)

Docker requires the memory swap limit to be updated in the same `docker update` call as the memory limit. Sablier handles this automatically by setting `MemorySwap` equal to `Memory`, which satisfies the constraint and disables swap for the container.
