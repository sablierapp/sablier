# Traefik Sablier Plugin

## Plugin

The plugin is available in the Traefik [Plugin Catalog](https://plugins.traefik.io/plugins/633b4658a4caa9ddeffda119/sablier) 

## Development

You can use this to load the plugin.

```yaml
version: "3.7"

services:
  traefik:
    image: traefik:2.9.1
    command:
      - --experimental.localPlugins.sablier.moduleName=github.com/acouvreur/sablier
      - --entryPoints.http.address=:80
      - --providers.docker=true
    ports:
      - "8080:80"
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
      - '../../..:/plugins-local/src/github.com/acouvreur/sablier'
      - './dynamic-config.yml:/etc/traefik/dynamic-config.yml'
```

But I recommend you to use the [`e2e`](./e2e/) folder.