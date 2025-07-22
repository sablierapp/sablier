# Podman

The Podman provider communicates with the `podman.sock` socket to start and stop containers on demand.

## Use the Podman provider

In order to use the docker provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: podman
```

#### **CLI**

```bash
sablier start --provider.name=podman
```

#### **Environment Variable**

```bash
PROVIDER_NAME=podman
```

<!-- tabs:end -->

!> **Ensure that Sablier has access to the docker socket!**

```yaml
services:
  sablier:
    image: sablierapp/sablier:1.9.0
    command:
      - start
      - --provider.name=podman
    volumes:
      - '/run/podman/podman.sock:/run/podman/podman.sock'
```

## Register containers

For Sablier to work, it needs to know which podman container to start and stop.

You have to register your containers by opting-in with labels.

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

## How does Sablier knows when a container is ready?

If the container defines a Healthcheck, then it will check for healthiness before stating the `ready` status.

If the containers do not define a Healthcheck, then as soon as the container has the status `started`