# Docker

The Docker provider communicates with the `docker.sock` socket to start and stop containers on demand.

## Use the Docker provider

In order to use the docker provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: docker
```

#### **CLI**

```bash
sablier start --provider.name=docker
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_NAME=docker
```

<!-- tabs:end -->

!> **Ensure that Sablier has access to the docker socket!**

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.11.1
    command:
      - start
      - --provider.name=docker
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```
<!-- x-release-please-end -->

## Register containers

For Sablier to work, it needs to know which docker container to start and stop.

You have to register your containers by opting-in with labels.

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

## Strategies

The Docker provider supports two strategies for managing containers:

### Stop Strategy (default)

The `stop` strategy completely stops containers when they become idle and starts them again when needed.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  docker:
    strategy: stop
```

#### **CLI**

```bash
sablier start --provider.docker.strategy=stop
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_DOCKER_STRATEGY=stop
```

<!-- tabs:end -->

### Pause Strategy

The `pause` strategy pauses containers instead of stopping them. This is faster than stop/start as the container state remains in memory, but uses more system resources.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  docker:
    strategy: pause
```

#### **CLI**

```bash
sablier start --provider.docker.strategy=pause
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_DOCKER_STRATEGY=pause
```

<!-- tabs:end -->

## How does Sablier knows when a container is ready?

If the container defines a Healthcheck, then it will check for healthiness before stating the `ready` status.

If the containers do not define a Healthcheck, then as soon as the container has the status `started`