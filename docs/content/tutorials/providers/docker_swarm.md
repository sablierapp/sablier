---
title: Docker Swarm
weight: 2
---

This tutorial connects Sablier to Docker Swarm. You will select the Docker Swarm provider, run Sablier with access to the Docker socket, register a service so Sablier can scale it, and confirm Sablier knows when the service is ready. The Docker Swarm provider communicates with the `docker.sock` socket to scale services on demand.

## Select the Docker Swarm provider

Set the [provider.name](/reference/cli/) property to `docker_swarm`.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: docker_swarm # or swarm
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.name=docker_swarm # or swarm
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=docker_swarm # or swarm
```

{{< /tab >}}
{{< /tabs >}}


{{< callout type="warning" >}}
**Ensure that Sablier has access to the docker socket!**
{{< /callout >}}

Run Sablier with the socket mounted:

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.14.0
    command:
      - start
      - --provider.name=docker_swarm # or swarm
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```
<!-- x-release-please-end -->

## Register a service

For Sablier to work, it needs to know which docker services to scale up and down. Register a service by opting in with labels:

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    deploy:
      labels:
        - sablier.enable=true
        - sablier.group=mygroup
```

## Confirm when the service is ready

Sablier checks for the service replicas. As soon as the current replicas matches the wanted replicas, then the service is considered `ready`.

{{< callout type="info" >}}
Docker Swarm uses the container's healthcheck to check if the container is up and running. So the provider has a native healthcheck support.
{{< /callout >}}