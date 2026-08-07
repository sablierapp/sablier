---
title: Labels & annotations
weight: 412
---

Sablier never modifies your application and keeps no central list of the workloads it manages. Instead, you **opt each workload in and configure it declaratively**, using the labelling mechanism your platform already provides. Sablier discovers what to manage by reading these labels from the [provider](/tutorials/providers/).

## Opt in with `sablier.enable`

A workload is managed by Sablier only when it carries the `sablier.enable=true` label. Without it, Sablier does not discover the workload and will not start or stop it.

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    labels:
      - "sablier.enable=true"
      - "sablier.group=demo"
```

## Configure behaviour with `sablier.*` labels

Everything beyond opting in is optional and expressed the same way. Labels control which [group(s)](/concepts/groups/) an instance belongs to, when it is kept warm, how readiness is decided, and how it scales:

- `sablier.group`: the group(s) this instance belongs to (see [Groups](/concepts/groups/)).
- `sablier.running-hours` / `sablier.running-days`: keep the instance warm on a schedule.
- `sablier.ready-after` / `sablier.ready-on-start`: tune when Sablier considers the instance ready.
- `sablier.idle.*` / `sablier.active.*`: throttle CPU, memory and block I/O instead of stopping (see [scale mode](/how-to-guides/scaling-resources/scale-mode/)).

The [Label reference](/reference/labels/) documents every key, its type and the version it was introduced in.

## Where labels live per provider

Each platform exposes labels differently, and Sablier reads them from the native location:

| Provider | Where Sablier reads configuration |
|----------|-----------------------------------|
| Docker, Podman | container `labels` |
| Docker Swarm | service `deploy.labels` |
| Kubernetes | workload `labels` for `sablier.enable`, `annotations` for every other key |
| Proxmox LXC | container `tags` |

{{< callout type="info" >}}
On **Kubernetes**, `sablier.enable` must be a *label* because workload discovery uses a label selector; multi-value settings such as `sablier.group` must be *annotations*. On **Proxmox LXC** there are no key/value labels, so Sablier reads *tags* like `sablier` and `sablier-group-<name>`.
{{< /callout >}}

See [Applying labels](/reference/labels/#applying-labels) for copy-paste examples in each provider's syntax.

## Labels take effect as workloads change

Sablier reads labels when it discovers workloads and as they start, stop or are updated, so adding or editing a label does not require restarting Sablier. Removing `sablier.enable` (or all of an instance's groups) simply drops the instance from Sablier's management.
