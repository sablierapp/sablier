# Docker Swarm

The Docker Swarm provider communicates with the `docker.sock` socket to scale services on demand.

## Use the Docker Swarm provider

In order to use the docker swarm provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: docker_swarm # or swarm
```

#### **CLI**

```bash
sablier start --provider.name=docker_swarm # or swarm
```

#### **Environment Variable**

```bash
PROVIDER_NAME=docker_swarm # or swarm
```

<!-- tabs:end -->


!> **Ensure that Sablier has access to the docker socket!**

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.10.5
    command:
      - start
      - --provider.name=docker_swarm # or swarm
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```
<!-- x-release-please-end -->

## Register services

For Sablier to work, it needs to know which docker services to scale up and down.

You have to register your services by opting-in with labels.

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    deploy:
      labels:
        - sablier.enable=true
        - sablier.group=mygroup
```

## How does Sablier know when a service is ready?

Sablier checks for the service replicas. As soon as the current replicas match the wanted replicas, then the service is considered `ready`.

?> Docker Swarm uses the container's healthcheck to check if the container is up and running. So the provider has native healthcheck support.

## Configuration Options

### Auto-stop on Startup

```yaml
provider:
  auto-stop-on-startup: true
```

When enabled, Sablier will scale down all services with `sablier.enable=true` label that have non-zero replicas but are not registered in an active session when Sablier starts.

## Service Labels

| Label | Required | Description | Example |
|-------|----------|-------------|---------|
| `sablier.enable` | Yes | Enable Sablier management for this service | `true` |
| `sablier.group` | Yes | Logical group name for the service | `myapp` |

**Important:** Labels must be in the `deploy` section for services, not at the service level.

## Full Example

See the [Docker Swarm provider example](../../examples/docker-swarm/) for a complete, working setup.

## Scaling Behavior

- Services start with 0 replicas
- On first request, Sablier scales to the last known replica count (default: 1)
- When session expires, Sablier scales back to 0
- Swarm automatically distributes replicas across nodes

## Limitations

- Requires Docker Swarm mode to be initialized
- Requires access to the Docker socket on a manager node
- Cannot scale global services (only replicated services)
- Services must use `replicated` mode, not `global`

## Troubleshooting

### Service not scaling

1. Check Sablier logs for errors
2. Verify the service has labels in the `deploy` section
3. Ensure Sablier is running on a manager node
4. Check service status: `docker service ps <service-name>`

### Sablier not starting

Ensure Sablier is deployed with a constraint to run on manager nodes:

```yaml
deploy:
  placement:
    constraints:
      - node.role == manager
```

### Services stuck in "preparing" state

Check if nodes have capacity and if images are available:

```bash
docker service ps <service-name>
docker node ls
```