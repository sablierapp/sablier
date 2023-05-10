# Docker Swarm

The Docker Swarm provider communicates with the `docker.sock` socket to scale services on demand.

## Use the Docker Swarm provider

In order to use the docker provider you can configure the [provider.name](TODO) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: docker_swarm
```

#### **CLI**

```bash
sablier start --provider.name=docker_swarm
```

#### **Environment Variable**

```bash
PROVIDER_NAME=docker_swarm
```

<!-- tabs:end -->


!> **Ensure that Sablier has access to the docker socket!**

```yaml
services:
  sablier:
    image: acouvreur/sablier:1.4.0-beta.3
    command:
      - start
      - --provider.name=docker_swarm
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```

## Register services

For Sablier to work, it needs to know which docker services to scale up and down.

You have to register your services by opting-in with labels.

```yaml
services:
  whoami:
    image: containous/whoami:v1.5.0
    deploy:
      labels:
        - sablier.enable=true
        - sablier.group=mygroup
```

## How does Sablier knows when a service is ready?

Sablier checks for the service replicas. As soon as the current replicas matches the wanted replicas, then the service is considered `ready`.

?> Docker Swarm uses the container's healthcheck to check if the container is up and running. So the provider has a native healthcheck support.