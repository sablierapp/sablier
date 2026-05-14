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
      --provider.docker.strategy string                       Strategy to use to stop docker containers (stop or pause) (default "stop")
      --provider.name string                                  Provider to use to manage containers [docker swarm kubernetes podman proxmox_lxc] (default "docker")
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
