---
title: Podman
weight: 4
---

This tutorial connects Sablier to Podman. You will select the Podman provider, run Sablier with access to the Podman socket, register a container so Sablier can manage it, and confirm Sablier knows when the container is ready. The Podman provider communicates with the `podman.sock` socket to start and stop containers on demand.

## Select the Podman provider

Set the [provider.name](/reference/cli/) property to `podman`.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: podman
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.name=podman
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=podman
```

{{< /tab >}}
{{< /tabs >}}

{{< callout type="warning" >}}
**Ensure that Sablier has access to the podman socket!**
{{< /callout >}}

Run Sablier with the socket mounted:

```yaml
services:
  sablier:
    image: sablierapp/sablier:{{< version >}}
    restart: always
    command:
      - start
      - --provider.name=podman
    volumes:
      - '/run/podman/podman.sock:/run/podman/podman.sock'
```

## Register a container

For Sablier to work, it needs to know which podman container to start and stop. Register a container by opting in with labels:

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    restart: unless-stopped
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

## Confirm when the container is ready

If the container defines a Healthcheck, then it will check for healthiness before stating the `ready` status.

If the containers do not define a Healthcheck, then as soon as the container has the status `started` it is considered `ready`.