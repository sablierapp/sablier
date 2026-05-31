# Configuration Reference

This page is generated automatically from the Sablier binary. Do not edit by hand.

All flags can be set via:

1. Command-line flag: `--flag.name value`
2. Environment variable: `FLAG_NAME=value` or `SABLIER_FLAG_NAME=value`
3. Config file (YAML): use the dotted flag name as nested YAML keys

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| <a id="configFile"></a>`--configFile` | `CONFIGFILE` / `SABLIER_CONFIGFILE` | `""` | Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory |
| <a id="logging-level"></a>`--logging.level` | `LOGGING_LEVEL` / `SABLIER_LOGGING_LEVEL` | `info` | The logging level. Can be one of [error, warn, info, debug] |
| <a id="provider-auto-stop-on-startup"></a>`--provider.auto-stop-on-startup` | `PROVIDER_AUTO_STOP_ON_STARTUP` / `SABLIER_PROVIDER_AUTO_STOP_ON_STARTUP` | `true` |  |
| <a id="provider-docker-strategy"></a>`--provider.docker.strategy` | `PROVIDER_DOCKER_STRATEGY` / `SABLIER_PROVIDER_DOCKER_STRATEGY` | `stop` | Strategy to use to stop docker containers (stop or pause) |
| <a id="provider-kubernetes-burst"></a>`--provider.kubernetes.burst` | `PROVIDER_KUBERNETES_BURST` / `SABLIER_PROVIDER_KUBERNETES_BURST` | `10` | Maximum burst for K8S API acees client-side throttling |
| <a id="provider-kubernetes-delimiter"></a>`--provider.kubernetes.delimiter` | `PROVIDER_KUBERNETES_DELIMITER` / `SABLIER_PROVIDER_KUBERNETES_DELIMITER` | `_` | Delimiter used for namespace/resource type/name resolution. Defaults to "_" for backward compatibility. But you should use "/" or "." |
| <a id="provider-kubernetes-qps"></a>`--provider.kubernetes.qps` | `PROVIDER_KUBERNETES_QPS` / `SABLIER_PROVIDER_KUBERNETES_QPS` | `5` | QPS limit for K8S API access client-side throttling |
| <a id="provider-name"></a>`--provider.name` | `PROVIDER_NAME` / `SABLIER_PROVIDER_NAME` | `docker` | Provider to use to manage containers [docker docker_swarm swarm kubernetes podman] |
| <a id="provider-podman-uri"></a>`--provider.podman.uri` | `PROVIDER_PODMAN_URI` / `SABLIER_PROVIDER_PODMAN_URI` | `unix:///run/podman/podman.sock` | Uri is the URI to connect to the Podman service. |
| <a id="server-base-path"></a>`--server.base-path` | `SERVER_BASE_PATH` / `SABLIER_SERVER_BASE_PATH` | `/` | The base path for the API |
| <a id="server-port"></a>`--server.port` | `SERVER_PORT` / `SABLIER_SERVER_PORT` | `10000` | The server port to use |
| <a id="sessions-default-duration"></a>`--sessions.default-duration` | `SESSIONS_DEFAULT_DURATION` / `SABLIER_SESSIONS_DEFAULT_DURATION` | `5m0s` | The default session duration |
| <a id="sessions-expiration-interval"></a>`--sessions.expiration-interval` | `SESSIONS_EXPIRATION_INTERVAL` / `SABLIER_SESSIONS_EXPIRATION_INTERVAL` | `20s` | The expiration checking interval. Higher duration gives less stress on CPU. If you only use sessions of 1h, setting this to 5m is a good trade-off. |
| <a id="storage-file"></a>`--storage.file` | `STORAGE_FILE` / `SABLIER_STORAGE_FILE` | `""` | File path to save the state |
| <a id="strategy-blocking-default-refresh-frequency"></a>`--strategy.blocking.default-refresh-frequency` | `STRATEGY_BLOCKING_DEFAULT_REFRESH_FREQUENCY` / `SABLIER_STRATEGY_BLOCKING_DEFAULT_REFRESH_FREQUENCY` | `5s` | Default refresh frequency at which the instances status are checked for blocking strategy |
| <a id="strategy-blocking-default-timeout"></a>`--strategy.blocking.default-timeout` | `STRATEGY_BLOCKING_DEFAULT_TIMEOUT` / `SABLIER_STRATEGY_BLOCKING_DEFAULT_TIMEOUT` | `1m0s` | Default timeout used for blocking strategy |
| <a id="strategy-dynamic-custom-themes-path"></a>`--strategy.dynamic.custom-themes-path` | `STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH` / `SABLIER_STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH` | `""` | Custom themes folder, will load all .html files recursively |
| <a id="strategy-dynamic-default-refresh-frequency"></a>`--strategy.dynamic.default-refresh-frequency` | `STRATEGY_DYNAMIC_DEFAULT_REFRESH_FREQUENCY` / `SABLIER_STRATEGY_DYNAMIC_DEFAULT_REFRESH_FREQUENCY` | `5s` | Default refresh frequency in the HTML page for dynamic strategy |
| <a id="strategy-dynamic-default-theme"></a>`--strategy.dynamic.default-theme` | `STRATEGY_DYNAMIC_DEFAULT_THEME` / `SABLIER_STRATEGY_DYNAMIC_DEFAULT_THEME` | `hacker-terminal` | Default theme used for dynamic strategy |
| <a id="strategy-dynamic-show-details-by-default"></a>`--strategy.dynamic.show-details-by-default` | `STRATEGY_DYNAMIC_SHOW_DETAILS_BY_DEFAULT` / `SABLIER_STRATEGY_DYNAMIC_SHOW_DETAILS_BY_DEFAULT` | `true` | Show the loading instances details by default |
