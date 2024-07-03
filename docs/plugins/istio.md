# Istio Plugin

The Istio Plugin is a WASM Plugin written with the Proxy Wasm SDK.

## Provider compatibility grid

| Provider                                | Dynamic | Blocking |
|-----------------------------------------|:-------:|:--------:|
| [Docker](../providers/docker)             |    ❌    |    ❌     |
| [Docker Swarm](../providers/docker_swarm) |    ❌    |    ❌     |
| [Kubernetes](../providers/kubernetes)     |    ✅    |    ✅     |

## Configuration

You can have the following configuration:

!> This only works for ingress gateways.
!> Attaching this filter to a side-car would not work because the side-car itself gets shutdown on scaling to zero. 

```yaml
apiVersion: extensions.istio.io/v1alpha1
kind: WasmPlugin
metadata:
  name: sablier-wasm-whoami-dynamic
  namespace: istio-system
spec:
  selector:
    matchLabels:
      istio: ingressgateway
  url: file:///opt/filters/sablierproxywasm.wasm/..data/sablierproxywasm.wasm
  # Use https://istio.io/latest/docs/reference/config/proxy_extensions/wasm-plugin/#WasmPlugin-TrafficSelector
  # To specify which service to apply this filter only
  phase: UNSPECIFIED_PHASE
  pluginConfig:
    {
      "sablier_url": "sablier.sablier-system.svc.cluster.local",
      "cluster": "outbound|10000||sablier.sablier-system.svc.cluster.local",
      "names": [ "deployment_default_whoami_1" ],
      "session_duration": "1m",
      "dynamic": {
        "display_name": "Dynamic Whoami",
        "theme": "hacker-terminal"
      }
    }
```