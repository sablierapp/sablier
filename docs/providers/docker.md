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
PROVIDER_NAME=docker
```

<!-- tabs:end -->

!> **Ensure that Sablier has access to the docker socket!**

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.10.1
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

## How does Sablier know when a container is ready?

If the container defines a Healthcheck, then Sablier will check for healthiness before marking the container as `ready`.

If the container does not define a Healthcheck, then as soon as the container has the status `started`, it is considered ready.

## Configuration Options

### Auto-stop on Startup

```yaml
provider:
  auto-stop-on-startup: true
```

When enabled, Sablier will stop all containers with `sablier.enable=true` label that are running but not registered in an active session when Sablier starts.

## Container Labels

| Label | Required | Description | Example |
|-------|----------|-------------|---------|
| `sablier.enable` | Yes | Enable Sablier management for this container | `true` |
| `sablier.group` | Yes | Logical group name for the container | `myapp` |

## Full Example

See the [Docker provider example](../../examples/docker/) for a complete, working setup.

## Limitations

- Requires access to the Docker socket
- Cannot manage containers in remote Docker hosts (use Docker Swarm for multi-host scenarios)
- Healthchecks must be defined in the container image or compose file

## Troubleshooting

### Container not starting

1. Check Sablier logs for errors
2. Verify the container has the correct labels
3. Ensure Sablier has access to the Docker socket

### Permission denied

Sablier needs read/write access to `/var/run/docker.sock`. Ensure the Sablier container has the socket mounted:

```yaml
volumes:
  - '/var/run/docker.sock:/var/run/docker.sock'
```

### Container starts but Sablier doesn't detect it

If your container has a healthcheck, ensure it's passing. Check with:

```bash
docker inspect <container-name> | grep Health -A 10
```