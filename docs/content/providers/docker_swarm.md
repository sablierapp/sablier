---
title: Docker Swarm
weight: 2
since: v1.0.0
aliases:
  - /tutorials/providers/docker_swarm/
---

## Introduction

The Docker Swarm provider scales [replicated services](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/#replicated-and-global-services) to zero [replicas](https://docs.docker.com/reference/compose-file/deploy/#replicas) when their session expires, and back up when a request needs them. Sablier runs as a [service](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/) on a [manager node](https://docs.docker.com/engine/swarm/how-swarm-mode-works/nodes/#manager-nodes) and calls the Docker API through the node's [Docker socket](https://docs.docker.com/reference/cli/dockerd/#daemon-socket-option).

## Install Sablier

Deploy Sablier as a [stack](https://docs.docker.com/engine/swarm/stack-deploy/) from a [manager node](https://docs.docker.com/engine/swarm/how-swarm-mode-works/nodes/#manager-nodes):

```bash
docker stack deploy -c sablier.yml sablier
```

{{< example file="docker-swarm/sablier.yml" >}}

- **Run Sablier on a manager node.** Only managers serve the service API, so the [placement constraint](https://docs.docker.com/reference/compose-file/deploy/#placement) keeps Sablier there.
- **Mount the manager's [Docker socket](https://docs.docker.com/reference/cli/dockerd/#daemon-socket-option).** Sablier lists, inspects and scales services through it.

Reverse proxy plugins reach Sablier at `http://sablier:10000`. Attach your reverse proxy to the `sablier` [overlay network](https://docs.docker.com/engine/network/drivers/overlay/) to resolve that name.

## Usage example

Opt a [service](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/) in with the `sablier.enable` [service label](https://docs.docker.com/reference/compose-file/deploy/#labels), and give it a group:

{{< example file="docker-swarm/whoami.yml" >}}

Deploy it with `docker stack deploy -c whoami.yml demo`. Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of the service. On the first request, Sablier scales the service to one [replica](https://docs.docker.com/reference/compose-file/deploy/#replicas) and holds the request, or shows a waiting page, until its [task](https://docs.docker.com/engine/swarm/how-swarm-mode-works/swarm-task-states/) passes its [healthcheck](https://docs.docker.com/reference/compose-file/services/#healthcheck). When the session expires, Sablier scales it back to zero.

{{< callout type="info" >}}
Sablier scales opted-in services to zero when it starts. A service you deploy afterwards keeps running until its first session expires, unless you [stop externally-started instances](/how-to-guides/lifecycle/auto-stop-externally-started/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Value | Description |
|---|---|---|
| [`--provider.name`](/reference/cli/#opt-provider-name) | `docker_swarm` or `swarm` | Selects the Docker Swarm provider. |

The provider has no options of its own. Sablier connects to Docker with the [standard Docker environment variables](https://docs.docker.com/reference/cli/docker/#environment-variables), such as `DOCKER_HOST`. Without them, it uses the socket at `/var/run/docker.sock`.

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/).

### Workloads

#### Replicated services

Sablier scales [replicated services](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/#replicated-and-global-services) by updating their [replica](https://docs.docker.com/reference/compose-file/deploy/#replicas) count:

- **Stop** scales to [`sablier.idle.replicas`](#scale-instead-of-stop), `0` by default.
- **Start** scales to [`sablier.active.replicas`](#scale-instead-of-stop), `1` by default.

Start ignores `deploy.replicas`. To run several replicas, set `sablier.active.replicas` to the same count:

{{< example file="docker-swarm/replicas.yml" region="sablier" >}}

### Instance names

A reverse proxy can target one service by name instead of a group. The `whoami` service of the usage example, deployed in the `demo` stack, is named:

```text
demo_whoami
```

The instance name is the exact [service](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/) name, as `docker service ls` lists it. [`docker stack deploy`](https://docs.docker.com/engine/swarm/stack-deploy/#deploy-the-stack-to-the-swarm) prefixes each service with the stack name and `_`.

Prefer groups: a name ties your proxy configuration to the stack name.

### Labels

On Docker Swarm, Sablier settings are `sablier.*` [service labels](https://docs.docker.com/reference/compose-file/deploy/#labels), under `deploy.labels`:

{{< example file="docker-swarm/group.yml" region="sablier" >}}

- **Put the labels under `deploy.labels`.** Sablier reads the service's labels only. The service-level [`labels`](https://docs.docker.com/reference/compose-file/services/#labels) key sets container labels, which Sablier ignores on Swarm.
- **`sablier.enable` must be exactly `true`.** Sablier finds services with a `sablier.enable=true` label filter.

The [label reference](/reference/labels/) lists every key.

{{< details title="Services created without a stack file" closed="true" >}}
With `docker service create`, set the keys with [`--label`](https://docs.docker.com/reference/cli/docker/service/create/#label), which sets service labels. `--container-label` sets container labels, which Sablier ignores.
{{< /details >}}

### Workload settings

Each excerpt below is the start of a complete stack file in [`examples/docker-swarm`](https://github.com/sablierapp/sablier/tree/main/examples/docker-swarm).

#### Enable {since="v1.4.0"}

`sablier.enable=true` opts the service in. It is required, must be a [service label](https://docs.docker.com/reference/compose-file/deploy/#labels), and any other value is ignored. See the [usage example](#usage-example).

#### Group {since="v1.4.0"}

`sablier.group` puts the service in one or more comma-separated groups. A session for any of them starts it. Without it, the service is in the `default` group.

{{< example file="docker-swarm/group.yml" region="sablier" >}}

[Learn more about groups](/how-to-guides/groups/).

#### Ready after {since="v1.13.0"}

`sablier.ready-after` keeps reporting the service as not ready for a settling delay after all its [tasks](https://docs.docker.com/engine/swarm/how-swarm-mode-works/swarm-task-states/) first run. Use it when a task passes its [healthcheck](https://docs.docker.com/reference/compose-file/services/#healthcheck) before it can serve traffic.

{{< example file="docker-swarm/ready-after.yml" region="sablier" >}}

[Learn more about delaying readiness](/how-to-guides/readiness/ready-after/).

#### Ready on start {since="v1.15.0"}

`sablier.ready-on-start=true` reports the service ready as soon as Sablier sends the scale-up, without waiting for its [tasks](https://docs.docker.com/engine/swarm/how-swarm-mode-works/swarm-task-states/).

{{< example file="docker-swarm/ready-on-start.yml" region="sablier" >}}

[Learn more about marking ready on start](/how-to-guides/readiness/ready-on-start/).

#### Running hours {since="v1.13.0"}

- `sablier.running-hours` keeps the service running during a daily window.
- `sablier.running-days` limits that window to some weekdays, since v1.15.0.

{{< example file="docker-swarm/running-hours.yml" region="sablier" >}}

The window uses Sablier's local time. Set the time zone with the `TZ` [environment variable](https://docs.docker.com/reference/compose-file/services/#environment) of the Sablier service:

{{< example file="docker-swarm/sablier.yml" region="timezone" >}}

[Learn more about running hours](/how-to-guides/lifecycle/running-hours/).

#### Anti-affinity {since="v1.15.0"}

`sablier.anti-affinity` lists groups the service backs off from: Sablier keeps it idle while any of them has an active session.

{{< example file="docker-swarm/anti-affinity.yml" region="sablier" >}}

[Learn more about anti-affinity](/how-to-guides/anti-affinity/).

#### Scale instead of stop {since="v1.13.0"}

Keep the service running when idle, optionally throttled, instead of scaling it to zero:

- `idle.replicas` set to `1` or more keeps that many replicas when idle.
- `idle.cpu` and `idle.memory` set the CPU and memory [limits](https://docs.docker.com/reference/compose-file/deploy/#resources) while idle.
- The matching `active.*` labels restore them on the next session.

CPU is in decimal cores, such as `0.5`. Memory uses Docker units, such as `64m` or `1g`.

{{< example file="docker-swarm/scale-mode.yml" region="sablier" >}}

{{< callout type="warning" >}}
Changing limits updates the service's task template, so Swarm replaces its [tasks](https://docs.docker.com/engine/swarm/how-swarm-mode-works/swarm-task-states/) with a [rolling update](https://docs.docker.com/engine/swarm/swarm-tutorial/rolling-update/). Sablier changes limits, never reservations. An idle limit stays in place on the next session unless you set its `active` counterpart.
{{< /callout >}}

[Learn more about scaling instead of stopping](/how-to-guides/scaling-resources/scale-mode/).

### Readiness

Sablier compares the desired and running [tasks](https://docs.docker.com/engine/swarm/how-swarm-mode-works/swarm-task-states/) of a service, and reports it:

- **stopped** when it has no desired task: it is scaled to `0`.
- **ready** when all desired tasks are running.
- **starting** otherwise.

Swarm marks a task running only once its container passes its [healthcheck](https://docs.docker.com/reference/compose-file/services/#healthcheck). Give your services one: without it, a task runs as soon as its container starts.

### Limitations

- Sablier must reach the Docker API of a [manager node](https://docs.docker.com/engine/swarm/how-swarm-mode-works/nodes/#manager-nodes): worker nodes do not serve the service API.
- Only [replicated services](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/#replicated-and-global-services) are supported, not global services or [jobs](https://docs.docker.com/reference/compose-file/deploy/#mode).
- Start always scales to `sablier.active.replicas`, one replica by default, whatever `deploy.replicas` says.
- A service is ready only when all its desired tasks run: no option accepts the first ready task.
- Sablier does not start [`depends_on`](https://docs.docker.com/reference/compose-file/services/#depends_on) services. Put dependencies in the same group instead.
- [Block I/O throttling](/how-to-guides/scaling-resources/scale-blkio/) is not available on Docker Swarm.
- [`docker stack deploy`](https://docs.docker.com/reference/cli/docker/stack/deploy/) sets each service back to its `deploy.replicas`, or one replica when unset. Redeploying a stack starts its stopped services, and Sablier treats them as [started externally](/how-to-guides/lifecycle/auto-stop-externally-started/).

## Examples

- [`examples/docker-swarm`](https://github.com/sablierapp/sablier/tree/main/examples/docker-swarm): every stack file on this page, with instructions to run them.
