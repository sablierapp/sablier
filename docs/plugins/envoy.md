# Envoy Plugin

The Envoy Plugin is a WASM Plugin written with the Proxy Wasm SDK.

## Provider compatibility grid

| Provider                                | Dynamic | Blocking |
|-----------------------------------------|:-------:|:--------:|
| [Docker](/providers/docker)             |    ✅    |    ✅     |
| [Docker Swarm](/providers/docker_swarm) |    ❓    |    ❓     |
| [Kubernetes](/providers/kubernetes)     |    ❓    |    ❓     |

## Configuration

You can have the following configuration:

```yaml
http_filters:
  - name: sablier-wasm-whoami-dynamic
    disabled: true
    typed_config:
      "@type": type.googleapis.com/udpa.type.v1.TypedStruct
      type_url: type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
      value:
        config:
          name: "sablier-wasm-whoami-dynamic"
          root_id: "sablier-wasm-whoami-dynamic"
          configuration:
            "@type": "type.googleapis.com/google.protobuf.StringValue"
            value: |
              {
                "sablier_url": "sablier:10000",
                "cluster": "sablier",
                "names": ["docker_classic_e2e-whoami-1"],
                "session_duration": "1m",
                "dynamic": {
                  "display_name": "Dynamic Whoami",
                  "theme": "hacker-terminal"
                }
              }
          vm_config:
            runtime: "envoy.wasm.runtime.v8"
            vm_id: "vm.sablier.sablier-wasm-whoami-dynamic"
            code:
              local:
                filename: "/etc/sablierproxywasm.wasm"
            configuration: { }
```