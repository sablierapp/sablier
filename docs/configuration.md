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
  # Provider to use to manage containers (docker, swarm, kubernetes)
  name: docker
  docker:
    # Strategy to use for stopping Docker containers: stop or pause (default: stop)
    strategy: stop
server:
  # The server port to use
  port: 10000 
  # The base path for the API
  base-path: /
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

## Environment Variables

All configuration options can be set as environment variables. The variable names follow the structure of the configuration file.

For example, this configuration:

```yaml
strategy:
  dynamic:
    custom-themes-path: /my/path
```

Becomes:

```bash
STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH=/my/path
```

## Arguments

To get the list of all available arguments:

<!-- x-release-please-start-version -->
```bash
sablier --help

# or

docker run sablierapp/sablier:1.10.5 --help
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
      --provider.name string                                  Provider to use to manage containers [docker swarm kubernetes] (default "docker")
      --server.base-path string                               The base path for the API (default "/")
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
