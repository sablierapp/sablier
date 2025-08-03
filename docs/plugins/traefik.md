# Traefik Sablier Plugin

The [Traefik Sablier Plugin](https://plugins.traefik.io/plugins/633b4658a4caa9ddeffda119/sablier) in the plugin catalog.

## Provider compatibility grid

| Provider                                | Dynamic |                        Blocking                         |
| --------------------------------------- | :-----: | :-----------------------------------------------------: |
| [Docker](../providers/docker)             |    ✅    |                            ✅                            |
| [Docker Swarm](../providers/docker_swarm) |    ✅    |                            ✅                            |
| [Kubernetes](../providers/kubernetes)     |    ✅    |                            ✅                            |

## Prerequisites

<!-- tabs:start -->

#### **Docker**

**Traefik will evict containers from its pool if they are stopped.**

Meaning labels attached to containers to autodiscover them is not possible with this plugin.

You have to use the dynamic config file provider instead.

**❌ You cannot do the following:**

```yaml
whoami:
  image: acouvreur/whoami:v1.10.2
  labels:
    - traefik.enable
    - traefik.http.routers.whoami.rule=PathPrefix(`/whoami`)
    - traefik.http.routers.whoami.middlewares=my-sablier@file
```

**✅ You should do the following instead:**

```yaml
http:
  services:
    whoami:
      loadBalancer:
        servers:
          - url: "http://whoami:80"

  routers:
    whoami:
      rule: PathPrefix(`/whoami`)
      entryPoints:
        - "http"
      middlewares:
        - my-sablier@file
      service: "whoami"
```
*dynamic-config.yaml*


#### **Docker Swarm**

**Traefik will evict services from its pool if they have 0 replicas.**

In order to use service labels, you have to add the following option on top of each services that will use this plugin.

See also [`traefik.docker.lbswarm`](https://doc.traefik.io/traefik/routing/providers/swarm/#traefikdockerlbswarm) label

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    deploy:
      replicas: 0
      labels:
        - traefik.docker.lbswarm=true
```

Traefik also have [allowEmptyServices](https://doc.traefik.io/traefik/providers/docker/#allowemptyservices) option which can be used instead.

#### **Kubernetes**

**Traefik will evict deployments from its pool if they have 0 endpoints available.**

You must use [`allowEmptyServices`](https://doc.traefik.io/traefik/providers/kubernetes-ingress/#allowemptyservices)

The blocking strategy is supported by issuing redirect which force client to retry request. It might fail if client do not support redirections (e.g. `curl` without `-L`). The limitation is caused by Traefik architecture. Everytime the underlying configuration changes, the whole router is regenrated, thus changing the router during a request will still map to the old router. For more details, see [#62](https://github.com/sablierapp/sablier/issues/62).

<!-- tabs:end -->

## Install the plugin to Traefik

<!-- tabs:start -->

#### **File (YAML)**

```yaml
experimental:
  plugins:
    sablier:
      moduleName: "github.com/sablierapp/sablier"
      version: "v1.10.1"
```

#### **CLI**

```bash
--experimental.plugins.sablier.modulename=github.com/sablierapp/sablier
--experimental.plugins.sablier.version=v1.10.1
```

<!-- tabs:end -->

## Configure the plugin using the Dynamic Configuration

<!-- tabs:start -->

#### **File (YAML)**

```yaml
http:
  middlewares:
    my-sablier:
      plugin:
        sablier:
          group: default
          dynamic:
            displayName: My Title
            refreshFrequency: 5s
            showDetails: "true"
            theme: hacker-terminal
          sablierUrl: http://sablier:10000
          sessionDuration: 1m
```

#### **Kubernetes CRD**

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: my-sablier
  namespace: my-namespace
spec:
  plugin:
    sablier:
      group: default
      dynamic:
        displayName: My Title
        refreshFrequency: 5s
        showDetails: "true"
        theme: hacker-terminal
      sablierUrl: http://sablier:10000
      sessionDuration: 1m
```

<!-- tabs:end -->

## Configuration reference

TODO
