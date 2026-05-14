# Configuration

There are three ways to define configuration options in Sablier:

1. In a configuration file
2. As environment variables
3. As command-line arguments

These methods are evaluated in the order listed above, with later methods overriding earlier ones.

If no value is provided for a given option, a default value is used.

## Instance Labels

Instance labels are applied directly to your containers or workloads. They control how Sablier discovers and manages each instance.

| Label | Required | Example | Description |
|-------|----------|---------|-------------|
| `sablier.enable` | Yes | `"true"` | Opt the instance into Sablier management. Any value other than `"true"` is ignored. |
| `sablier.group` | No | `"myapp"` | Assign the instance to a named group. Defaults to `"default"` when `sablier.enable=true` and no group is set. |
| `sablier.ready-after` | No | `"30s"` | Minimum duration to wait after the instance first reports ready before Sablier considers it truly ready. Accepts any Go duration string (`"500ms"`, `"1m30s"`, …). Useful for services that are started or pass their health check before they can actually serve traffic. |
| `sablier.running-hours` | No | `"09:00-18:00"` | Daily keep-warm window in local time (`HH:MM-HH:MM`). Sablier starts the instance at window start and keeps it running until window end. Overnight windows like `"22:00-06:00"` are supported. |
| `sablier.idle.cpu` | No | `"0.1"` | CPU limit applied when the session expires (scale mode). Requires `sablier.idle.replicas >= 1`. |
| `sablier.idle.memory` | No | `"128m"` | Memory limit applied when the session expires (scale mode). Requires `sablier.idle.replicas >= 1`. |
| `sablier.idle.replicas` | No | `"1"` | Replica count when idle. Default `0` (stop the workload). Set to `1` or higher to keep the workload running and optionally throttle its resources. |
| `sablier.active.cpu` | No | `"2.0"` | CPU limit restored when a new session is requested (scale mode). |
| `sablier.active.memory` | No | `"512m"` | Memory limit restored when a new session is requested (scale mode). |
| `sablier.active.replicas` | No | `"1"` | Replica count when active. Default `1`. Increase when you need more replicas on wake-up. |

### `sablier.ready-after`

Some services are started (or pass their health check) before they finish initialising — for example a JVM application that opens its HTTP port before loading all caches, a database that accepts TCP connections before it's ready for queries, or any container without a health check that needs a few extra seconds after start-up before it can serve traffic.

Setting `sablier.ready-after` introduces a mandatory settling delay. Once the provider reports the instance as ready — whether that means it has started or passed its health check — Sablier continues to return a *not-ready* response to any blocking or dynamic request until the grace period elapses.

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.ready-after=30s"  # wait 30 s after started/healthy before unblocking requests
```

The value is a Go duration string. Valid examples:

| Value | Duration |
|-------|----------|
| `500ms` | 500 milliseconds |
| `30s` | 30 seconds |
| `1m30s` | 1 minute 30 seconds |
| `2m` | 2 minutes |

If the label is absent or set to an unparseable value, no extra wait is applied.

!> The `sablier.ready-after` grace period counts from when the instance **first** becomes ready in a given session. It does not reset on subsequent requests.

### `sablier.running-hours`

Use `sablier.running-hours` when an instance must stay available during specific daily hours.

Behavior:

- At the beginning of the configured period, Sablier proactively starts the instance.
- During the running-hours period, request-triggered sessions are extended to the period end so the instance is not stopped in the middle of the window.
- After the period ends, normal session expiration resumes.

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.running-hours=09:00-18:00"
```

Format rules:

- Use 24-hour format `HH:MM-HH:MM`.
- If start is later than end (for example `22:00-06:00`), the window spans midnight.
- If the label cannot be parsed, Sablier ignores it.

### Timezone (`TZ`)

Running-hours are evaluated in the process local timezone.

- In the official Docker image, the binary embeds timezone database data and supports `TZ` out of the box.
- The container defaults to `TZ=UTC`.
- Override timezone with environment variables, for example `-e TZ=Europe/Paris`.

### Scale Mode

Scale mode is an alternative to stopping containers. Instead of shutting down and restarting the workload, Sablier **scales down the replica count** when the session expires and **restores it** when a new session is requested.

Optionally, when `sablier.idle.replicas >= 1`, the workload **keeps running** at the idle replica count with **throttled CPU and memory**, and resources are **restored** when a new session arrives. This eliminates cold-start latency at the cost of keeping the workload alive.

| Label | Format | Default | Example |
|-------|--------|---------|----------|
| `sablier.idle.replicas` | Integer | `0` (stop) | `"1"` |
| `sablier.idle.cpu` | Decimal cores (Docker/Swarm) or Kubernetes quantity | — | `"0.1"`, `"500m"` |
| `sablier.idle.memory` | Docker units or Kubernetes quantity | — | `"64m"`, `"128Mi"` |
| `sablier.active.replicas` | Integer | `1` | `"2"` |
| `sablier.active.cpu` | Decimal cores (Docker/Swarm) or Kubernetes quantity | — | `"2.0"`, `"2000m"` |
| `sablier.active.memory` | Docker units or Kubernetes quantity | — | `"512m"`, `"1Gi"` |

When `sablier.idle.replicas` is `0` (the default), Sablier stops the workload on session expiry and restarts it on demand — identical to the default stop behavior, but configured via scale-mode labels. Set it to `1` or higher to keep the workload running with optional resource throttling.

You can set any combination of the CPU and memory labels. Labels not set default to no limit (no throttling applied).

**Supported providers:** Docker, Docker Swarm, Kubernetes (Deployments and StatefulSets), Podman.

> Proxmox LXC does not support scale mode because it uses tag-based configuration rather than key-value labels.

#### Docker / Podman

CPU values are decimal fractions of one core (`"0.5"` = half a core). Memory values use Docker-style suffixes (`b`, `k`, `m`, `g`).

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.idle.replicas=1"
      - "sablier.idle.cpu=0.1"
      - "sablier.idle.memory=64m"
      - "sablier.active.replicas=1"
      - "sablier.active.cpu=2.0"
      - "sablier.active.memory=512m"
```

When the session expires, Sablier sets the replica count to `sablier.idle.replicas` (≥ 1 keeps the container running) and runs the equivalent of `docker update --cpus=0.1 --memory=64m myapp`. When a new session is requested, it restores `--cpus=2.0 --memory=512m`. The container is never stopped.

> **Note:** Docker requires the memory swap limit to be updated in the same call as the memory limit. Sablier sets `MemorySwap` equal to `Memory` automatically, which satisfies the constraint and disables swap for the container.

#### Docker Swarm

CPU and memory values use the same format as Docker. Resource constraints are applied to the service's task template, triggering a Swarm service update (tasks are re-scheduled with the new limits).

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
        - "sablier.idle.cpu=0.1"
        - "sablier.idle.memory=64m"
        - "sablier.active.replicas=1"
        - "sablier.active.cpu=2.0"
        - "sablier.active.memory=512m"
```

> **Note:** In Docker Swarm, labels that control Sablier must be placed under `deploy.labels`, not the top-level `labels` key, so that they are attached to the service rather than the container.

#### Kubernetes

CPU and memory values use the standard [Kubernetes resource quantity](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-units-in-kubernetes) format (`"500m"`, `"2"`, `"128Mi"`, `"1Gi"`).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    sablier.enable: "true"
    sablier.group: myapp
    sablier.idle.replicas: "1"
    sablier.idle.cpu: "100m"
    sablier.idle.memory: "64Mi"
    sablier.active.replicas: "1"
    sablier.active.cpu: "2000m"
    sablier.active.memory: "512Mi"
```

> **Note:** Kubernetes resource limit changes trigger a **rolling restart** of the pods. The service remains available during the transition (old pods are replaced with new ones), but there will be brief overlap between old and new pods.

!> Scale mode changes resource **limits**, not requests. Ensure your nodes have sufficient allocatable capacity for the active limits.

## Configuration File

At startup, Sablier searches for a configuration file named `sablier.yml` (or `sablier.yaml`) in the following locations:

- `/etc/sablier/`
- `$XDG_CONFIG_HOME/`
- `$HOME/.config/`
- `.` *(the working directory)*

You can override this by using the `configFile` argument:

```bash
sablier --configFile=path/to/myconfigfile.yml
```

```yaml
provider:
  # Provider to use to manage containers (docker, swarm, kubernetes, podman, proxmox_lxc)
  name: docker
  # Stop all sablier.enable=true instances that are running at startup and were not started by Sablier (default: true)
  auto-stop-on-startup: true
  # Continuously stop instances with sablier.enable=true that are running but were not started by Sablier (default: false)
  # Uses both event-driven detection and a periodic reconciliation scan (every 30 seconds) as a safety net.
  auto-stop-externally-started: false
  # Reject direct named requests for instances without sablier.enable=true
  reject-unlabeled-requests: false
  # Verify sablier.enable=true before stopping expired instances
  verify-enabled-on-expiration: false
  docker:
    # Strategy to use for stopping Docker containers: stop or pause (default: stop)
    strategy: stop
server:
  # The server port to use
  port: 10000 
  # The base path for the API
  base-path: /
  metrics:
    # Expose a Prometheus /metrics endpoint at <base-path>/metrics (default: false)
    enabled: false
storage:
  # File path to save the state (default stateless)
  file:
sessions:
  # The default session duration (default 5m)
  default-duration: 5m
  # The expiration checking interval. 
  # Higher duration gives less stress on CPU. 
  # If you only use sessions of 1h, setting this to 5m is a good trade-off.
  expiration-interval: 20s
logging:
  level: debug
strategy:
  dynamic:
    # Custom themes folder, will load all .html files recursively (default empty)
    custom-themes-path:
    # Show instances details by default in waiting UI
    show-details-by-default: false
    # Default theme used for dynamic strategy (default "hacker-terminal")
    default-theme: hacker-terminal
    # Default refresh frequency in the HTML page for dynamic strategy
    default-refresh-frequency: 5s
  blocking:
    # Default timeout used for blocking strategy (default 1m)
    default-timeout: 1m
```

## server.metrics.enabled

| Key | Default | Description |
|-----|---------|-------------|
| `server.metrics.enabled` | `false` | Expose a Prometheus-compatible `/metrics` endpoint. |

When set to `true`, Sablier registers a `GET <base-path>/metrics` route on the same HTTP server that handles `/health` and `/api/...`. The route returns the Prometheus text exposition format and is served by `promhttp`. When set to `false` (the default), the route is not registered and any request to that path returns `404`.

### Exposed metrics

| Name | Type | Labels | Description |
|------|------|--------|-------------|
| `sablier_group_locked` | gauge | `group` | `1` if any instance in the group has an active session, else `0`. One series per known group, including groups with no active sessions. |
| `sablier_group_active_instances` | gauge | `group` | Number of instances in the group that currently have an active session. |
| `sablier_instance_start_duration_seconds` | histogram | `instance` | Duration of `provider.InstanceStart` calls (seconds). Observed only on success. |
| `sablier_instance_ready_duration_seconds` | histogram | `instance` | End-to-end wall time from first not-ready observation to ready (seconds). |
| `sablier_session_requests_total` | counter | `strategy` (`dynamic`\|`blocking`), `target` (`names`\|`group`) | Total number of session requests received. |
| `sablier_instance_start_failures_total` | counter | `instance` | Total number of `provider.InstanceStart` failures. |
| `sablier_instance_stops_total` | counter | `instance`, `reason` (`expired`\|`unregistered`) | Total number of instance stops. |
| Go runtime + process collectors | (default) | (default) | Standard `go_*` and `process_*` metrics from the Prometheus Go client. |

### Security note

The endpoint exposes process internals, group and instance names, and counters. It is intended for trusted observability stacks. Restrict at the reverse proxy when Sablier is fronted on an untrusted network.

## Environment Variables

All configuration options can be set as environment variables. The variable names follow the structure of the configuration file and are prefixed with `SABLIER_`.

For example, this configuration:

```yaml
strategy:
  dynamic:
    custom-themes-path: /my/path
```

Becomes:

```bash
SABLIER_STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH=/my/path
```

## Arguments

To get the list of all available arguments:

<!-- x-release-please-start-version -->
```bash
sablier --help

# or

docker run sablierapp/sablier:1.12.0 --help
```
<!-- x-release-please-end -->

All configuration options can be used as command-line arguments. The argument names follow the structure of the configuration file.

For example, this configuration:

```yaml
strategy:
  dynamic:
    custom-themes-path: /my/path
```

Becomes:

```bash
sablier start --strategy.dynamic.custom-themes-path /my/path
```

## Reference

```
  -h, --help                                                  help for start
      --provider.auto-stop-on-startup                         Stop all sablier.enable=true instances running at startup that were not started by Sablier (default true)
  --provider.auto-stop-externally-started                 Continuously stop instances with sablier.enable=true that are running but were not started by Sablier (default false)
      --provider.docker.strategy string                       Strategy to use to stop docker containers (stop or pause) (default "stop")
      --provider.name string                                  Provider to use to manage containers [docker swarm kubernetes podman proxmox_lxc] (default "docker")
      --provider.reject-unlabeled-requests                    Reject requests for instances without sablier.enable=true (default false)
      --provider.verify-enabled-on-expiration                 Verify sablier.enable=true before stopping expired instances (default false)
      --server.base-path string                               The base path for the API (default "/")
      --server.metrics.enabled                                Enable the Prometheus /metrics endpoint (default false)
      --server.port int                                       The server port to use (default 10000)
      --sessions.default-duration duration                    The default session duration (default 5m0s)
      --sessions.expiration-interval duration                 The expiration checking interval. Higher duration gives less stress on CPU. If you only use sessions of 1h, setting this to 5m is a good trade-off. (default 20s)
      --storage.file string                                   File path to save the state
      --strategy.blocking.default-timeout duration            Default timeout used for blocking strategy (default 1m0s)
      --strategy.dynamic.custom-themes-path string            Custom themes folder, will load all .html files recursively
      --strategy.dynamic.default-refresh-frequency duration   Default refresh frequency in the HTML page for dynamic strategy (default 5s)
      --strategy.dynamic.default-theme string                 Default theme used for dynamic strategy (default "hacker-terminal")
      --strategy.dynamic.show-details-by-default              Show the loading instances details by default (default true)
```
