---
title: Connect a provider
description: "Connect Sablier to your platform: Docker, Docker Swarm, Kubernetes, Podman or Proxmox LXC."
weight: 40
---

Each provider below is a step-by-step tutorial that wires Sablier to one platform, from selecting the provider to confirming a workload wakes up on demand. Pick the platform you run and follow it start to finish.

{{< cards cols="3" >}}
  {{< card link="/tutorials/providers/docker/" image="/assets/img/docker.svg" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Docker" subtitle="Stop and start containers on demand." >}}
  {{< card link="/tutorials/providers/docker_swarm/" image="/assets/img/docker_swarm.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Docker Swarm" subtitle="Scale services to zero and back on demand." >}}
  {{< card link="/tutorials/providers/kubernetes/" image="/assets/img/kubernetes.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Kubernetes" subtitle="Scale Deployments and StatefulSets to zero." >}}
  {{< card link="/tutorials/providers/podman/" image="/assets/img/podman.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Podman" subtitle="Stop and start containers on demand." >}}
  {{< card link="/tutorials/providers/proxmox_lxc/" image="/assets/img/proxmox.png" imageStyle="object-fit:contain;height:120px;padding:24px;background:#ffffff;" title="Proxmox LXC" subtitle="Stop and start LXC containers via the Proxmox VE API." >}}
{{< /cards >}}

*Your Provider is not on the list? [Open an issue to request the missing provider here!](https://github.com/sablierapp/sablier/issues/new?assignees=&labels=enhancement%2C+provider&projects=&template=instance-provider-request.md&title=Add+%60%5BPROVIDER%5D%60+provider)*

[See the active issues about providers](https://github.com/sablierapp/sablier/issues?q=is%3Aopen+is%3Aissue+label%3Aprovider)