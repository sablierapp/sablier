# CLI Reference

This page is generated automatically from the Sablier binary. Do not edit by hand.

## `sablier`

Sablier is an API that starts containers on demand.
It provides integrations with multiple reverse proxies and different loading strategies.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

### `sablier completion`

Generate the autocompletion script for sablier for the specified shell.
See each sub-command's help for details on how to use the generated script.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

#### `sablier completion bash`

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(sablier completion bash)

To load completions for every new session, execute once:

#### Linux:

	sablier completion bash > /etc/bash_completion.d/sablier

#### macOS:

	sablier completion bash > $(brew --prefix)/etc/bash_completion.d/sablier

You will need to start a new shell for this setup to take effect.

**Usage:**

```
sablier completion bash
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--no-descriptions` | `false` | disable completion descriptions |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

#### `sablier completion fish [flags]`

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	sablier completion fish | source

To load completions for every new session, execute once:

	sablier completion fish > ~/.config/fish/completions/sablier.fish

You will need to start a new shell for this setup to take effect.

**Usage:**

```
sablier completion fish [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--no-descriptions` | `false` | disable completion descriptions |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

#### `sablier completion powershell [flags]`

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	sablier completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.

**Usage:**

```
sablier completion powershell [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--no-descriptions` | `false` | disable completion descriptions |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

#### `sablier completion zsh [flags]`

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(sablier completion zsh)

To load completions for every new session, execute once:

#### Linux:

	sablier completion zsh > "${fpath[1]}/_sablier"

#### macOS:

	sablier completion zsh > $(brew --prefix)/share/zsh/site-functions/_sablier

You will need to start a new shell for this setup to take effect.

**Usage:**

```
sablier completion zsh [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--no-descriptions` | `false` | disable completion descriptions |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

### `sablier docs [flags]`

Generate documentation artifacts from the binary itself.

Outputs:
  {output}/reference/cli.md       - CLI reference (all commands and flags)
  {output}/reference/config.md    - Configuration and environment variable reference
  {output}/reference/themes.md    - Theme template variable reference
  {output}/sablier.sample.yaml    - Sample configuration file with all defaults

The generated files are intended to be committed to the repository and consumed
by the MkDocs documentation build.

**Usage:**

```
sablier docs [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--help` | `false` | help for docs |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--output` | `docs/generated` | Directory to write generated documentation into |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

### `sablier health [flags]`

Calls the health endpoint of a Sablier instance

**Usage:**

```
sablier health [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--url` | `http://localhost:10000/health` | Sablier health endpoint |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

### `sablier help [command]`

Help provides help for any command in the application.
Simply type sablier help [path to command] for full details.

**Usage:**

```
sablier help [command]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

### `sablier start [flags]`

Start the Sablier server

**Usage:**

```
sablier start [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--provider.auto-stop-on-startup` | `true` |  |
| `--provider.docker.strategy` | `stop` | Strategy to use to stop docker containers (stop or pause) |
| `--provider.kubernetes.burst` | `10` | Maximum burst for K8S API acees client-side throttling |
| `--provider.kubernetes.delimiter` | `_` | Delimiter used for namespace/resource type/name resolution. Defaults to "_" for backward compatibility. But you should use "/" or "." |
| `--provider.kubernetes.qps` | `5` | QPS limit for K8S API access client-side throttling |
| `--provider.name` | `docker` | Provider to use to manage containers [docker docker_swarm swarm kubernetes podman] |
| `--provider.podman.uri` | `unix:///run/podman/podman.sock` | Uri is the URI to connect to the Podman service. |
| `--server.base-path` | `/` | The base path for the API |
| `--server.port` | `10000` | The server port to use |
| `--sessions.default-duration` | `5m0s` | The default session duration |
| `--sessions.expiration-interval` | `20s` | The expiration checking interval. Higher duration gives less stress on CPU. If you only use sessions of 1h, setting this to 5m is a good trade-off. |
| `--storage.file` | `""` | File path to save the state |
| `--strategy.blocking.default-refresh-frequency` | `5s` | Default refresh frequency at which the instances status are checked for blocking strategy |
| `--strategy.blocking.default-timeout` | `1m0s` | Default timeout used for blocking strategy |
| `--strategy.dynamic.custom-themes-path` | `""` | Custom themes folder, will load all .html files recursively |
| `--strategy.dynamic.default-refresh-frequency` | `5s` | Default refresh frequency in the HTML page for dynamic strategy |
| `--strategy.dynamic.default-theme` | `hacker-terminal` | Default theme used for dynamic strategy |
| `--strategy.dynamic.show-details-by-default` | `true` | Show the loading instances details by default |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

### `sablier version`

Print the version Sablier

**Usage:**

```
sablier version
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| `--configFile` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| `--logging.level` | `info` | The logging level. Can be one of [error, warn, info, debug] |

