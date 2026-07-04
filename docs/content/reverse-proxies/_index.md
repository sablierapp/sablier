---
title: Reverse proxies
weight: 60
---

## What is a Reverse Proxy Plugin?

Reverse proxy plugins provide integration with a reverse proxy.

{{< callout type="info" >}}
Because Sablier is designed as an API that can be used independently, reverse proxy integrations act as clients of that API.
{{< /callout >}}

They leverage API calls to intercept in-flight requests and communicate with Sablier.

![Reverse Proxy Integration](/assets/img/reverse-proxy-integration.png)

{{< cards cols="3" >}}
  {{< card link="/reverse-proxies/traefik/" icon="puzzle" title="Traefik" >}}
  {{< card link="/reverse-proxies/caddy/" icon="puzzle" title="Caddy" >}}
  {{< card link="/reverse-proxies/nginx_proxywasm/" icon="puzzle" title="Nginx" >}}
  {{< card link="/reverse-proxies/envoy/" icon="puzzle" title="Envoy" >}}
  {{< card link="/reverse-proxies/istio/" icon="puzzle" title="Istio" >}}
  {{< card link="/reverse-proxies/apacheapisix/" icon="puzzle" title="Apache APISIX" >}}
{{< /cards >}}

## Available Reverse Proxies

| Reverse Proxy                 | Docker | Docker Swarm mode | Kubernetes |
| ----------------------------- | :----: | :---------------: | :--------: |
| [Apache APISIX](/reverse-proxies/apacheapisix/) |   ✅    |         ✅         |     ✅      |
| [Caddy](/reverse-proxies/caddy/)                |   ✅    |         ✅         |     ❌      |
| [Envoy](/reverse-proxies/envoy/)                |   ✅    |         ❓         |     ❓      |
| [Istio](/reverse-proxies/istio/)                |   ❌    |         ❌         |     ⚠️      |
| [Nginx](/reverse-proxies/nginx_proxywasm/)      |   ✅    |         ❓         |     ❓      |
| [Traefik](/reverse-proxies/traefik/)            |   ✅    |         ✅         |     ✅      |
| [ProxyWasm](/reverse-proxies/nginx_proxywasm/)  |   ✅    |         ✅         |     ✅      |

> ✅ **Fully compatible**
> 
> ⚠️ **Partially compatible**
> 
> ❓ **Should be compatible (but not tested)**
> 
> ❌ **Not compatible**

*Your Reverse Proxy is not on the list? [Open an issue to request the missing reverse proxy integration here!](https://github.com/sablierapp/sablier/issues/new?assignees=&labels=enhancement%2C+reverse-proxy&projects=&template=reverse-proxy-integration-request.md&title=Add+%60%5BREVERSE+PROXY%5D%60+reverse+proxy+integration)*

## Runtime and Compiled Plugins

Some reverse proxies can evaluate plugins at runtime (e.g., Traefik with Yaegi, NGINX with Lua and JavaScript plugins), which means the reverse proxy can consume the plugin directly without recompilation.

Others require you to rebuild your reverse proxy with the plugin included (e.g., Caddy).