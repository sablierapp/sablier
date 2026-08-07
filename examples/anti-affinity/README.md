# Anti-Affinity Example

This example demonstrates Sablier's **anti-affinity**: making one workload back off automatically while another is in use.

It targets the classic problem of several heavy services competing for a **shared, non-shareable resource** — most often **GPU VRAM or RAM**. Running two of them at once causes an out-of-memory crash or a severe slowdown. Anti-affinity ensures that when one service is requested, the others yield.

## How It Works

An instance declares the groups it must yield to with the `sablier.anti-affinity` label:

```yaml
labels:
  - "sablier.enable=true"
  - "sablier.group=background"
  - "sablier.anti-affinity=streaming"   # yield whenever "streaming" is active
```

- When a session for the **`streaming`** group becomes active, every instance that declared an anti-affinity against it is forced to its **idle** state.
  - Without scale-mode labels, "idle" means the container is **stopped**.
  - With scale-mode labels (`sablier.idle.replicas=1`, `sablier.idle.cpu=…`), "idle" means the **idle resource profile** is applied and the container keeps running throttled.
- When the `streaming` session **expires** and no other listed group is active, the instances Sablier forced idle are **restored**.

List multiple antagonist groups comma-separated (`sablier.anti-affinity=streaming,transcoding`); the instance stays idle while **any** of them is active.

## This Example

| Service | Labels | Behaviour |
|---------|--------|-----------|
| `streaming` | `sablier.group=streaming` | The high-priority service. Requesting it activates the `streaming` group. |
| `background` | `sablier.anti-affinity=streaming` | Runs normally, but is forced idle (stopped here) while `streaming` is active, then restored. |

## Running the Example

```bash
# Start everything
make up

# Activate background, request streaming, watch background yield, then restore
make demo

# Tear down
make down
```

## Manual Steps

```bash
docker compose up -d

# 1. Activate the background service (gives it a live session)
curl 'http://localhost:10000/api/strategies/blocking?group=background&timeout=30s'
docker compose ps            # background is running

# 2. Request the streaming service — background must yield
curl 'http://localhost:10000/api/strategies/blocking?group=streaming&timeout=30s'
sleep 3
docker compose ps -a         # background is now exited (forced idle)

# 3. Let the streaming session expire, then check again
sleep 40
docker compose ps            # background is running again (restored)
```

## Notes

- Only instances Sablier **actually suppressed while running** are restored later. An instance that was already idle is left untouched — that is why the demo activates `background` first.
- The relationship is **one-directional**: `background` yields to `streaming`, not the reverse.
- While `streaming` is active, requesting `background` does **not** start it — it is reported as `not-ready` with a message explaining it is paused by anti-affinity (shown on the waiting page and in the API response). It starts automatically once `streaming` expires.
- Anti-affinity instances must be **Sablier-managed** (`sablier.enable=true`) so Sablier can stop and start them.
- To keep a background service **always on** except while an antagonist is active, combine anti-affinity with [`sablier.running-hours`](../../docs/configuration.md#sablierrunning-hours) or auto-warm, so it re-acquires a session after being restored.
- Anti-affinity is supported on **Docker**, **Docker Swarm**, **Kubernetes**, and **Podman** (not Proxmox LXC).

📚 **[Full documentation](../../docs/configuration.md#anti-affinity)**
