# Traefik Sablier Plugin

- [Traefik Sablier Plugin](#traefik-sablier-plugin)
  - [Installation](#installation)
  - [Traefik with Docker classic](#traefik-with-docker-classic)
  - [Traefik with Docker Swarm](#traefik-with-docker-swarm)
  - [Traefik with Kubernetes](#traefik-with-kubernetes)
  - [Plugin](#plugin)
  - [Development](#development)

## Installation

1. Add this snippet in the Traefik Static configuration

```yaml
experimental:
  plugins:
    sablier:
      moduleName: "github.com/acouvreur/sablier"
      version: "v1.3.0"
```

2. Configure the plugin using the Dynamic Configuration. Example:

```yaml
http:
  middlewares:
    my-sablier:
      plugin:
        sablierUrl: http://sablier:10000  # The sablier URL service, must be reachable from the Traefik instance
        names: whoami,nginx               # Comma separated names of containers/services/deployments etc.
        sessionDuration: 1m               # The session duration after which containers/services/deployments instances are shutdown
        # You can only use one strategy at a time
        # To do so, only declare `dynamic` or `blocking`

        # Dynamic strategy, provides the waiting webui
        dynamic:
          displayName: My Title       # (Optional) Defaults to the middleware name
          showDetails: true           # (Optional) Set to true or false to show details specifcally for this middleware, unset to use Sablier server defaults
          theme: hacker-terminal      # (Optional) The theme to use
          refreshFrequency: 5s        # (Optional) The loading page refresh frequency

        # Blocking strategy, waits until services are up and running
        # but will not wait more than `timeout`
        # blocking: 
        #   timeout: 1m
```

You can also checkout the End to End tests here: [e2e](./e2e/).

## Traefik with Docker classic


**Container labels cannot be used**

Traefik will evict the container from its pool if it's not running. Which means the middleware won't trigger on incoming request.

You must use the dynamic configuration to route to the container whatever the container state is.

**Example**

*docker-compose.yml*
```yaml
version: "3.9"

services:
  traefik:
    image: traefik:2.9.1
    command:
      - --entryPoints.http.address=:80
      - --providers.docker=true
      - --providers.file.filename=/etc/traefik/dynamic-config.yml
      - --experimental.plugins.sablier.moduleName=github.com/acouvreur/sablier/plugins/traefik
      - --experimental.plugins.sablier.version=v1.3.0
    ports:
      - "8080:80"
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
      - './dynamic-config.yml:/etc/traefik/dynamic-config.yml'

  sablier:
    image: acouvreur/sablier:1.3.0
    command:
      - start
      - --provider.name=docker
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
    labels:
      - traefik.enable=true
      # Dynamic Middleware
      - traefik.http.middlewares.dynamic.plugin.sablier.names=sablier-whoami-1
      - traefik.http.middlewares.dynamic.plugin.sablier.sablierUrl=http://sablier:10000
      - traefik.http.middlewares.dynamic.plugin.sablier.dynamic.sessionDuration=1m

  whoami:
    image: containous/whoami:v1.5.0
    # Cannot use labels because as soon as the container is stopped, the labels are not treated by Traefik
    # The route doesn't exist anymore. Use dynamic-config.yml file instead.
    # labels:
    #  - traefik.enable
    #  - traefik.http.routers.whoami-dynamic.rule=PathPrefix(`/dynamic/whoami`)
    #  - traefik.http.routers.whoami-dynamic.middlewares=dynamic@docker
```

*dynamic-config.yaml*
```yaml
http:
  services:
    whoami:
      loadBalancer:
        servers:
        - url: "http://whoami:80"

  routers:
    whoami-dynamic:
      rule: PathPrefix(`/dynamic/whoami`)
      entryPoints:
        - "http"
      middlewares:
        - dynamic@docker
      service: "whoami"
```

## Traefik with Docker Swarm

⚠️ Limitations

- Traefik will evict the service from its pool as soon as the service is 0/0. You must add the [`traefik.docker.lbswarm`](https://doc.traefik.io/traefik/routing/providers/docker/#traefikdockerlbswarm) label.
    ```yaml
    services:
      whoami:
        image: containous/whoami:v1.5.0
        deploy:
          replicas: 0
          labels:
            - traefik.docker.lbswarm=true
    ```
- We cannot use [allowEmptyServices](https://doc.traefik.io/traefik/providers/docker/#allowemptyservices) because if you use the [blocking strategy](LINKHERE) you will receive a `503`.

## Traefik with Kubernetes

- The format of the `names` section is `<KIND>_<NAMESPACE>_<NAME>_<REPLICACOUNT>` where `_` is the delimiter.
  - Thus no `_` are allowed in `<NAME>`
- `KIND` can be either `deployment` or `statefulset`

⚠️ Limitations

- Traefik will evict the service from its pool as soon as there is no endpoint available. You must use [`allowEmptyServices`](https://doc.traefik.io/traefik/providers/kubernetes-ingress/#allowemptyservices)
- Blocking Strategy is not yet supported because of how Traefik handles the pod ip.

See [Kubernetes E2E Traefik Test script](./e2e/kubernetes.sh) to see how it is reproduced

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