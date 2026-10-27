---
title: Reverse proxies
description: "Integrate Sablier with your reverse proxy: Traefik, Caddy, Nginx, Envoy, Istio or Apache APISIX."
weight: 60
aliases:
  - /tutorials/reverse-proxies/
---

{{< cards cols="3" >}}
  {{< card link="/reverse-proxies/traefik/" image="/assets/img/traefik.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Traefik" subtitle="Runtime middleware plugin (Yaegi)." >}}
  {{< card link="/reverse-proxies/caddy/" image="/assets/img/caddy.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Caddy" subtitle="Handler module, compiled into Caddy." >}}
  {{< card link="/reverse-proxies/nginx_proxywasm/" image="/assets/img/nginx.svg" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Nginx" subtitle="ProxyWasm plugin." >}}
  {{< card link="/reverse-proxies/envoy/" image="/assets/img/envoy.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Envoy" subtitle="ProxyWasm plugin." >}}
  {{< card link="/reverse-proxies/istio/" image="/assets/img/istio.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Istio" subtitle="ProxyWasm plugin via EnvoyFilter." >}}
  {{< card link="/reverse-proxies/apacheapisix/" image="/assets/img/apacheapisix.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Apache APISIX" subtitle="ProxyWasm plugin." >}}
{{< /cards >}}

*Your Reverse Proxy is not on the list? [Open an issue to request the missing reverse proxy integration here!](https://github.com/sablierapp/sablier/issues/new?assignees=&labels=enhancement%2C+reverse-proxy&projects=&template=reverse-proxy-integration-request.md&title=Add+%60%5BREVERSE+PROXY%5D%60+reverse+proxy+integration)*
