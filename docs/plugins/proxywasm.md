# ProxyWasm Sablier Plugin

A WASM Sablier Plugin that uses the [ProxyWasm Go SDK](https://github.com/tetratelabs/proxy-wasm-go-sdk).

## Provider compatibility grid

| Provider                                | Dynamic | Blocking |
| --------------------------------------- | :-----: | :------: |
| [Docker](/providers/docker)             |    ✅    |    ✅     |
| [Docker Swarm](/providers/docker_swarm) |    ✅    |    ✅     |
| [Kubernetes](/providers/kubernetes)     |    ✅    |    ✅     |

## Install the plugin

In order to install the WASM Filter, you need to check with your reverse proxy current support for WASM and which ABI Specification it uses. For example, some reverse proxy may use [http-wasm](https://http-wasm.io/) instead.

You can retrieve the precompiled WASM Plugin in the [Github Release](https://github.com/acouvreur/sablier/releases) page.

## Configuration

```json
{
  "sablier_url": "sablier:10000", 
  "names": [
    "docker_classic_e2e-whoami-1"
  ],
  "session_duration": "1m", 
  "dynamic": { 
    "display_name": "Dynamic Whoami", 
    "theme": "hacker-terminal"
  }
}
```

## Examples

Check the current examples in the plugin folder: [examples](https://github.com/acouvreur/sablier/tree/main/plugins/proxywasm/examples).