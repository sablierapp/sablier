# Configuration

There are three ways to define configuration options in Sablier:

1. In a configuration file
2. As environment variables
3. As command-line arguments

These methods are evaluated in the order listed above, with later methods overriding earlier ones.

If no value is provided for a given option, a default value is used.

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
  # Ignore instances without sablier.enable=true during start, stop, and event operations
  ignore-unlabeled: false
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

docker run sablierapp/sablier:1.11.2 --help
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
      --provider.ignore-unlabeled                             Ignore instances without sablier.enable=true during start, stop, and event operations
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
