# Apache APISIX Plugin

The Apache APISIX Plugin is a WASM Plugin written with the Proxy Wasm SDK.

## Provider compatibility grid

| Provider                               | Dynamic | Blocking |
|----------------------------------------|:-------:|:--------:|
| [Docker](../providers/docker)          |    ✅    |    ✅     |
| [Docker Swarm](../providers/docker_swarm) |    ❓    |    ❓     |
| [Kubernetes](../providers/kubernetes)     |    ❓    |    ❓     |

## Install the plugin to Apache APISIX

```yaml
wasm:
  plugins:
    - name: proxywasm_sablier_plugin
      priority: 7997
      file: /wasm/sablierproxywasm.wasm # Downloaded WASM Filter path
```

## Configuration

You can have the following configuration:

```yaml
routes:
  - uri: "/"
    plugins:
      proxywasm_sablier_plugin:
        conf: '{ "sablier_url": "sablier:10000", "group": ["my-group"], "session_duration": "1m", "dynamic": { "display_name": "Dynamic Whoami" } }'
```