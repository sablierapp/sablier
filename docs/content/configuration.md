---
title: Configuration
weight: 90
---

There are three ways to define configuration options in Sablier:

1. In a configuration file
2. As environment variables
3. As command-line arguments

These methods are evaluated in the order listed above, with later methods overriding earlier ones.

If no value is provided for a given option, a default value is used.

## Instance Labels

Instance labels are applied directly to your containers or workloads. They control how Sablier discovers and manages each instance.

The complete, auto-generated list — with types, defaults, examples and provider-specific notes — is in the [**Label reference**](/labels/). Each label's behavior is documented in depth under the [Features](/features/) section.

## Features

Per-instance behavior is configured with the labels above and documented feature by feature:

{{< cards >}}
  {{< card link="/features/scale-mode/" icon="chip" title="Scale mode" subtitle="Throttle CPU, memory and block I/O when idle." >}}
  {{< card link="/features/running-hours/" icon="clock" title="Running hours" subtitle="Keep an instance warm on a daily schedule." >}}
  {{< card link="/features/anti-affinity/" icon="scale" title="Anti-affinity" subtitle="Back off while another group is active." >}}
  {{< card link="/features/ready-after/" icon="clock" title="Ready after" subtitle="Settling delay before serving traffic." >}}
  {{< card link="/features/ready-on-start/" icon="lightning-bolt" title="Ready on start" subtitle="Skip the health check for background services." >}}
  {{< card link="/features/multiple-groups/" icon="puzzle" title="Multiple groups" subtitle="One instance in several groups." >}}
{{< /cards >}}

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
  # Continuously create a session (with the default session duration) for instances with sablier.enable=true
  # that are running but were not started by Sablier, instead of stopping them (default: false)
  # This is the non-destructive counterpart to auto-stop-externally-started (the two options are
  # mutually exclusive): the instance keeps running until its seeded session expires, then hibernates
  # through the regular expiration lifecycle. Uses both event-driven detection and a periodic
  # reconciliation scan (every 30 seconds) as a safety net.
  # Pair it with auto-stop-on-startup: false, otherwise instances already running when Sablier
  # boots are stopped once at startup before the warm watch takes over.
  auto-warm-externally-started: false
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

## Metrics

Enabling and reading the Prometheus `/metrics` endpoint is documented under [Metrics](/metrics/).

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

docker run sablierapp/sablier:1.14.0 --help
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

The complete, auto-generated list of command-line flags — with each flag's environment variable, type, default and description — lives in the [**CLI reference**](/cli/). It is generated from the code, so it never drifts.
