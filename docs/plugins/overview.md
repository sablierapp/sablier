# Reverse Proxy Plugins

## What is a Reverse Proxy Plugin ?

Reverse proxy plugins are the integration with a reverse proxy.

?> Because Sablier is designed as an API that can be used on its own, reverse proxy integrations acts as a client of that API.

It leverages the API calls to plugin integration to catch in-flight requests to Sablier.

![Reverse Proxy Integration](../assets/img/reverse-proxy-integration.png)

## Available Reverse Proxies

| Reverse Proxy                | Docker | Docker Swarm mode | Kubernetes |                          Podman                           |
| ---------------------------- | :----: | :---------------: | :--------: | :-------------------------------------------------------: |
| [Traefik](/plugins/traefik) |   ✅    |         ✅         |     ✅      | [See #70](https://github.com/acouvreur/sablier/issues/70) |
| [Nginx](/plugins/nginx)     |   ✅    |         ✅         |     ❌      |
| [Caddy](/plugins/caddy)     |   ✅    |         ✅         |     ❌      |

## Runtime and Compiled plugins

Some reverse proxies have the capability to evaluate the plugins at runtime (Traefik with Yaegi, NGINX with Lua and JS plugins) which means the reverse proxy provides a way to consume the plugin directly.

Some others enforce you to rebuild your reverse proxy (Caddy).