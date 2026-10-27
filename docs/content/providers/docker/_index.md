---
title: Docker
weight: 1
aliases:
  - /tutorials/providers/docker/
---

This tutorial connects Sablier to Docker. You will select the Docker provider, run Sablier with access to the Docker socket, register a container so Sablier can manage it, and confirm Sablier knows when the container is ready. The Docker provider communicates with the `docker.sock` socket to start and stop containers on demand.

## Select the Docker provider

Set the [provider.name](/reference/cli/) property to `docker`.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: docker
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.name=docker
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=docker
```

{{< /tab >}}
{{< /tabs >}}

{{< callout type="warning" >}}
**Ensure that Sablier has access to the docker socket.**
{{< /callout >}}

Run Sablier with the socket mounted:

```yaml
services:
  sablier:
    image: sablierapp/sablier:{{< version >}}
    restart: always
    command:
      - start
      - --provider.name=docker
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```

To limit what Sablier can do with the socket, [use a Docker socket proxy](/providers/docker/socket-proxy/).

## Connecting to a remote or TLS-protected daemon

By default Sablier connects to the local Docker socket. To reach a remote or TLS-protected daemon, set the standard Docker environment variables on the Sablier container; Sablier reads them through the Docker client.

| Variable | Purpose |
|----------|---------|
| `DOCKER_HOST` | Daemon socket or address, for example `tcp://127.0.0.1:2376`. |
| `DOCKER_API_VERSION` | Pin the Docker Engine API version instead of negotiating it. |
| `DOCKER_CERT_PATH` | Directory holding `ca.pem`, `cert.pem` and `key.pem` for a TLS connection. |
| `DOCKER_TLS_VERIFY` | Set to a non-empty value to verify the daemon's TLS certificate. |

These are the standard Docker CLI variables, so an environment that already works with the `docker` CLI works with Sablier unchanged.

## Register a container

For Sablier to work, it needs to know which docker container to start and stop. Register a container by opting in with labels:

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    restart: unless-stopped
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

## Strategies

The Docker provider supports two strategies for managing containers:

### Stop Strategy (default)

The `stop` strategy completely stops containers when they become idle and starts them again when needed.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  docker:
    strategy: stop
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.docker.strategy=stop
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_DOCKER_STRATEGY=stop
```

{{< /tab >}}
{{< /tabs >}}

### Pause Strategy

The `pause` strategy pauses containers instead of stopping them. This is faster than stop/start as the container state remains in memory, but uses more system resources.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  docker:
    strategy: pause
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.docker.strategy=pause
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_DOCKER_STRATEGY=pause
```

{{< /tab >}}
{{< /tabs >}}

## Confirm when the container is ready

If the container defines a Healthcheck, then it will check for healthiness before stating the `ready` status.

If the containers do not define a Healthcheck, then as soon as the container has the status `started`, it is considered `ready`.

## Docker Compose

The Docker provider reads the metadata `docker compose` writes on each container. See [Docker Compose](/providers/docker/compose/) for the Compose features Sablier supports, such as [starting `depends_on` dependencies together](/providers/docker/compose/depends-on/).

## One-shot containers

To report one-shot init or migration containers as `completed` instead of `stopped`, [honor the restart policy](/providers/docker/restart-policy/).

## Feature support

Which Sablier features work with the Docker provider. A ≈ mark means the feature works, but you configure it differently than on other providers. See the [compatibility matrix](/reference/compatibility/) to compare every provider.

{{< provider-features "docker" >}}
