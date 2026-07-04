---
title: Providers
weight: 50
---

## What is a Provider?

A Provider is how Sablier interacts with your instances.

A Provider typically has the following capabilities:
- Start an instance
- Stop an instance
- Get the current status of an instance
- Listen for instance lifecycle events (started, stopped)

{{< cards cols="3" >}}
  {{< card link="/providers/docker/" icon="server" title="Docker" >}}
  {{< card link="/providers/docker_swarm/" icon="server" title="Docker Swarm" >}}
  {{< card link="/providers/kubernetes/" icon="server" title="Kubernetes" >}}
  {{< card link="/providers/podman/" icon="server" title="Podman" >}}
  {{< card link="/providers/proxmox_lxc/" icon="server" title="Proxmox LXC" >}}
{{< /cards >}}

## Available Providers

| Provider                     | Name                      | Details                                                          |
|------------------------------|---------------------------|------------------------------------------------------------------|
| [Docker](/providers/docker/)             | `docker`                  | Stop and start **containers** on demand                          |
| [Docker Swarm](/providers/docker_swarm/) | `docker_swarm` or `swarm` | Scale down to zero and up **services** on demand                 |
| [Kubernetes](/providers/kubernetes/)     | `kubernetes`              | Scale down and up **deployments** and **statefulsets** on demand |
| [Podman](/providers/podman/)             | `podman`                  | Stop and start **containers** on demand                          |
| [Proxmox LXC](/providers/proxmox_lxc/)  | `proxmox_lxc`             | Stop and start **LXC containers** on demand via Proxmox VE API  |

*Your Provider is not on the list? [Open an issue to request the missing provider here!](https://github.com/sablierapp/sablier/issues/new?assignees=&labels=enhancement%2C+provider&projects=&template=instance-provider-request.md&title=Add+%60%5BPROVIDER%5D%60+provider)*

[See the active issues about providers](https://github.com/sablierapp/sablier/issues?q=is%3Aopen+is%3Aissue+label%3Aprovider)