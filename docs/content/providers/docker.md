---
title: Docker
weight: 1
since: v1.0.0
aliases:
  - /tutorials/providers/docker/
  - /features/startup-dependencies/
  - /features/startup-dependencies/depends-on/
  - /features/startup-dependencies/restart-policy/
  - /features/security/
  - /features/security/docker-socket-proxy/
---

## Introduction

The Docker provider stops idle [containers](https://docs.docker.com/get-started/docker-concepts/the-basics/what-is-a-container/) when their session expires, and starts them again when a request needs them. It can also pause them, or keep them running with throttled resources. Sablier runs as a container next to them and calls the [Docker Engine API](https://docs.docker.com/reference/api/engine/) through the [Docker socket](https://docs.docker.com/engine/daemon/). With [Docker Compose](https://docs.docker.com/compose/), it starts a container's [`depends_on`](https://docs.docker.com/reference/compose-file/services/#depends_on) dependencies first.

## Install Sablier

{{< tabs >}}
{{< tab name="Docker socket" >}}

Run Sablier as a [Compose service](https://docs.docker.com/reference/compose-file/services/), with the [Docker socket](https://docs.docker.com/engine/daemon/) mounted:

{{< example file="docker/compose.yml" >}}

```bash
docker compose up -d sablier
```

`docker` is the default provider, so `--provider.name=docker` is optional.

{{< callout type="warning" >}}
The socket gives [root-level access](https://docs.docker.com/engine/security/#docker-daemon-attack-surface) to the host. To limit what Sablier can do, use a socket proxy.
{{< /callout >}}

{{< /tab >}}
{{< tab name="Socket proxy" >}}

Put a [socket proxy](https://github.com/linuxserver/docker-socket-proxy) between Sablier and the [Docker socket](https://docs.docker.com/engine/daemon/), so Sablier reaches only the API endpoints it needs. Use this file instead of `compose.yml`:

{{< example file="docker/socket-proxy.yml" >}}

- **`CONTAINERS=1`** lets Sablier list and inspect containers.
- **`EVENTS=1` and `VERSION=1`** let it follow container events and check the connection at startup.
- **`POST=1` is required.** After a stop, Sablier waits for the container to exit, and no `ALLOW_*` flag covers that endpoint. `CONTAINERS=1` still scopes it to containers: images, networks and volumes stay blocked.
- **Only the proxy mounts the socket**, on an [internal network](https://docs.docker.com/reference/compose-file/networks/#internal) that nothing outside the host can reach.

The proxy answers `403 Forbidden` to every other endpoint.

{{< /tab >}}
{{< tab name="Remote daemon" >}}

Sablier reads the standard [Docker environment variables](https://docs.docker.com/reference/cli/docker/#environment-variables). Set them on its container to manage a remote [daemon](https://docs.docker.com/engine/daemon/), for example over [TLS](https://docs.docker.com/engine/security/protect-access/#use-tls-https-to-protect-the-docker-daemon-socket). Use this file instead of `compose.yml`:

{{< example file="docker/remote.yml" >}}

- **`DOCKER_HOST`** is the daemon address, such as `tcp://docker.example.com:2376`.
- **`DOCKER_CERT_PATH` turns TLS on.** The directory holds the client certificates, as in the example.
- **Set `DOCKER_TLS_VERIFY=1`.** Without it, Sablier doesn't verify the daemon's certificate.
- **`DOCKER_API_VERSION`** pins the API version instead of negotiating it. Use it for debugging only.

{{< details title="Differences from the docker CLI" closed="true" >}}
- Sablier doesn't read certificates from `~/.docker`. Without `DOCKER_CERT_PATH`, it connects without TLS, whatever `DOCKER_TLS_VERIFY` says.
- Sablier doesn't support `ssh://` hosts.
{{< /details >}}

{{< /tab >}}
{{< /tabs >}}

Reverse proxy plugins reach Sablier at `http://sablier:10000` from a container on the same [Compose network](https://docs.docker.com/compose/how-tos/networking/).

If the connection to the daemon drops, Sablier reconnects on its own. It waits one more second after each failed attempt, up to 30 seconds.

## Usage example

Opt a container in with the `sablier.enable` [label](https://docs.docker.com/engine/manage-resources/labels/), and give it a group:

{{< example file="docker/whoami.yml" >}}

Add it to the [Compose project](https://docs.docker.com/compose/how-tos/project-name/) that runs Sablier, and [create](https://docs.docker.com/reference/cli/docker/compose/create/) the container without starting it:

```bash
docker compose -f compose.yml -f whoami.yml create whoami
```

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of the `whoami` service. On the first request, Sablier starts the container and holds the request, or shows a waiting page, until its [health check](https://docs.docker.com/reference/compose-file/services/#healthcheck) passes. When the session expires, Sablier stops it.

{{< callout type="info" >}}
Sablier stops opted-in containers when it starts. A container you start afterwards, with `docker compose up`, `docker start` or a [restart policy](https://docs.docker.com/engine/containers/start-containers-automatically/), keeps running until its first session expires. To avoid that, [stop externally-started instances](/features/stop-external-starts/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.docker.strategy`](/reference/cli/#opt-provider-docker-strategy) | `stop` | `stop` or `pause` idle containers. See [Pause instead of stop](#pause-instead-of-stop). Since v1.11.0. |
| [`--provider.docker.honor-restart-policy`](/reference/cli/#opt-provider-docker-honor-restart-policy) | `false` | Report a one-shot container that exited `0` as completed. See [One-shot containers](#one-shot-containers). Deprecated: the default in v2. Since v1.15.0. |

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/). Sablier connects to the daemon with the [Docker environment variables](#install-sablier). With [tracing](/features/tracing/) on, every Docker API call becomes a span, with no daemon configuration.

### Workloads

#### Containers

Sablier starts and stops [containers](https://docs.docker.com/get-started/docker-concepts/the-basics/what-is-a-container/) that already exist:

- **Stop** runs the equivalent of [`docker stop`](https://docs.docker.com/reference/cli/docker/container/stop/), then waits for the container to exit. Docker sends the stop signal, then kills the container after its [stop grace period](https://docs.docker.com/reference/compose-file/services/#stop_grace_period).
- **Start** runs the equivalent of [`docker start`](https://docs.docker.com/reference/cli/docker/container/start/). A container that is already running is left alone.

Sablier never creates, recreates or removes a container. After [`docker compose down`](https://docs.docker.com/reference/cli/docker/compose/down/), create the containers again.

Give managed containers `restart: unless-stopped` or `restart: "no"`. With `always`, Docker restarts a container Sablier stopped [when the daemon restarts](https://docs.docker.com/engine/containers/start-containers-automatically/#restart-policy-details).

#### Pause instead of stop {since="v1.11.0"}

Set `--provider.docker.strategy=pause` to [pause](https://docs.docker.com/reference/cli/docker/container/pause/) idle containers instead of stopping them:

{{< example file="docker/pause.yml" >}}

- **Stop** frees the container's CPU and memory. The next request waits for the application to boot.
- **Pause** freezes the container's processes and keeps their memory. The next request resumes them instantly.

The strategy applies to every container. A container that is stopped rather than paused is started as usual.

### Docker Compose

Compose sets these [labels](https://docs.docker.com/engine/manage-resources/labels/) on every container it creates:

- **`com.docker.compose.project`:** the [project](https://docs.docker.com/compose/how-tos/project-name/) name.
- **`com.docker.compose.service`:** the [service](https://docs.docker.com/reference/compose-file/services/) name.
- **`com.docker.compose.depends_on`:** the service's dependencies, as `service:condition:restart` entries.

Sablier reads them to start a container's dependencies. A container created without Compose has no dependencies.

#### Start dependencies together {since="v1.15.0"}

Declare the order with [`depends_on`](https://docs.docker.com/reference/compose-file/services/#depends_on). When a request starts a container, Sablier starts its dependencies first:

{{< example file="docker/depends-on.yml" >}}

A request for the `app` group starts `db` and waits until it is healthy. It then starts `migration` and waits until it has completed. Only then does it start `app`. Sablier waits for each [condition](https://docs.docker.com/compose/how-tos/startup-order/):

| Condition | Sablier waits until the dependency is |
|---|---|
| `service_started` (default) | running, or has completed |
| `service_healthy` | running and [healthy](https://docs.docker.com/reference/compose-file/services/#healthcheck), or running without a health check |
| `service_completed_successfully` | completed: exited `0` under a non-restarting policy. Requires [the restart policy option](#one-shot-containers). |

Decide whether to label each dependency:

- **Dependencies need no labels to start.** Sablier starts every dependency a container declares, opted in or not.
- **Sablier only stops opted-in containers.** Give a dependency `sablier.enable=true` and the same group to stop it with the group. Leave it unlabeled to keep it running, for example a database that other apps share.
- **Never label a one-shot container.** A session is ready only when every member is ready, and a container that exits by design never is.
- **A dependency that already meets its condition is left alone.** Sablier doesn't restart a running database, and doesn't run a completed migration again.

{{< details title="How Sablier resolves dependencies" closed="true" >}}
- Sablier looks for dependencies in the same Compose project only. It skips the dependencies of a container without a project label, and logs a warning.
- It skips a dependency service that has no container, and logs a warning.
- When a service has several containers, it picks a running one, otherwise the first name in alphabetical order.
- It resolves dependencies of dependencies, and starts independent ones in parallel.
- If the graph has a cycle, it ignores the dependencies, logs a warning, and starts the container alone.
- A dependency that exits with an error, or is dead, fails the start.
- It ignores the `restart` option of `depends_on`.
- The whole start, dependencies included, must finish within five minutes.
- A dependency written without a condition, by older Compose versions, must be running, healthy or not, like Compose does. A one-shot that already exited does not count. Any unknown condition behaves like `service_started`.
{{< /details >}}

#### One-shot containers {since="v1.15.0"}

A one-shot container, such as a migration, runs once and exits `0`. For a `service_completed_successfully` dependency on it to resolve, set `--provider.docker.honor-restart-policy`:

{{< example file="docker/honor-restart-policy.yml" >}}

With it, Sablier reads the container's [restart policy](https://docs.docker.com/engine/containers/start-containers-automatically/) when it exits:

| Exit code | Restart policy | Status |
|---|---|---|
| `0` | `no` or `on-failure` | **completed**: the job is done. |
| `0` | `always` or `unless-stopped` | **stopped**: Sablier starts it again on demand. |
| Not `0` | any | **error** |

Without the option, every container that exits `0` is **stopped**. The condition never resolves, and the start times out.

- **A completed container doesn't run again.** Later requests find it completed and skip it, until you recreate it.
- **Give long-running containers `restart: unless-stopped`.** Docker reports a missing restart policy as `no`, so a container without one looks completed once Sablier stops it, if it exits `0`.

{{< callout type="warning" >}}
`--provider.docker.honor-restart-policy` is deprecated. It will be removed in v2, where honoring the restart policy becomes the default.
{{< /callout >}}

### Instance names

A reverse proxy can target one container by name instead of a group. The `whoami` container of the usage example is named:

```text
demo-whoami-1
```

The name is the container's name. Compose names containers `<project>-<service>-<index>`, unless you set a [`container_name`](https://docs.docker.com/reference/compose-file/services/#container_name).

Prefer groups: a name ties your proxy configuration to the project name.

### Labels

Sablier reads [labels](https://docs.docker.com/engine/manage-resources/labels/) on the container. In Compose, set them under the service's [`labels`](https://docs.docker.com/reference/compose-file/services/#labels), as a list or a map:

{{< example file="docker/group.yml" region="sablier" >}}

- **`sablier.enable` must be `true`.** Sablier lists containers with the filter `sablier.enable=true`.
- **Labels are fixed when the container is created.** Docker [can't change them](https://docs.docker.com/engine/manage-resources/labels/#manage-labels-on-objects) afterwards, so recreate the container after you edit them.
- **Don't use `deploy.labels`.** Compose sets them [on the service only](https://docs.docker.com/reference/compose-file/deploy/#labels), not on its containers.

The [label reference](/reference/labels/) lists every key.

### Workload settings

Each excerpt below comes from a Compose file in [`examples/docker`](https://github.com/sablierapp/sablier/tree/main/examples/docker). Add it to `compose.yml` with another `-f` flag.

#### Enable {since="v1.4.0"}

`sablier.enable=true` opts the container in. It is required, and any other value is ignored. See the [usage example](#usage-example).

#### Group {since="v1.4.0"}

`sablier.group` puts the container in one or more comma-separated groups. A session for any of them starts it. Without it, the container is in the `default` group.

{{< example file="docker/group.yml" region="sablier" >}}

[Learn more about groups](/features/groups/).

#### Ready after {since="v1.13.0"}

`sablier.ready-after` keeps reporting the container as not ready for a settling delay after its [health check](https://docs.docker.com/reference/compose-file/services/#healthcheck) first passes. Use it when the container passes its health check before it can serve traffic.

{{< example file="docker/ready-after.yml" region="sablier" >}}

[Learn more about delaying readiness](/features/ready-after/).

#### Ready on start {since="v1.15.0"}

`sablier.ready-on-start=true` reports the container ready as soon as Sablier starts it, without waiting for its [health check](https://docs.docker.com/reference/compose-file/services/#healthcheck).

{{< example file="docker/ready-on-start.yml" region="sablier" >}}

[Learn more about marking ready on start](/features/ready-on-start/).

#### Running hours {since="v1.13.0"}

- `sablier.running-hours` keeps the container running during a daily window.
- `sablier.running-days` limits that window to some weekdays, since v1.15.0.

{{< example file="docker/running-hours.yml" region="sablier" >}}

The window uses Sablier's local time. The Sablier image defaults to UTC: set the `TZ` [environment variable](https://docs.docker.com/reference/compose-file/services/#environment) to change it.

{{< example file="docker/timezone.yml" >}}

[Learn more about running hours](/features/running-hours/).

#### Anti-affinity {since="v1.15.0"}

`sablier.anti-affinity` lists groups the container backs off from: Sablier keeps it idle while any of them has an active session.

{{< example file="docker/anti-affinity.yml" region="sablier" >}}

[Learn more about anti-affinity](/features/anti-affinity/).

#### Scale instead of stop {since="v1.13.0"}

Keep the container running when idle, optionally throttled, instead of stopping it:

- `sablier.idle.replicas=1` keeps it running when idle.
- `idle.cpu` and `idle.memory` set its [CPU and memory limits](https://docs.docker.com/engine/containers/resource_constraints/) while idle.
- `active.cpu` and `active.memory` restore them on the next session.

{{< example file="docker/scale-mode.yml" region="sablier" >}}

Sablier changes the limits with the equivalent of [`docker update --cpus=0.1 --memory=64m`](https://docs.docker.com/reference/cli/docker/container/update/).

- **CPU is in cores**, as a decimal: `0.5` is half a core.
- **Memory uses Docker units:** `b`, `k`, `m` or `g`.
- **Set the active limits.** Docker keeps an idle limit until another update replaces it.
- **A memory limit disables swap.** Docker needs the swap limit updated with the memory limit, so Sablier sets both to the same value.
- **Start the container yourself.** In this mode, Sablier never starts or stops it, so use `docker compose up` instead of `create`.
- **Set active labels only with an idle replica count of `1` or more.** Otherwise Sablier stops the container when idle, but only updates its limits on the next session.

Replica counts above `1` and `sablier.active.replicas` have no effect: a container is one replica.

[Learn more about scaling instead of stopping](/features/scale-mode/).

#### Block I/O {since="v1.15.0"}

In scale mode, Sablier can also throttle the container's [block I/O](https://docs.docker.com/engine/containers/resource_constraints/) while idle:

{{< example file="docker/blkio.yml" region="sablier" >}}

| Labels (`idle` and `active`) | Value |
|---|---|
| `blkio-weight` | Relative weight from `10` to `1000`. Other values, including `0`, are ignored. |
| `blkio-weight-device` | `path:weight` |
| `blkio-device-read-bps`, `blkio-device-write-bps` | `path:rate`, in Docker byte units: `10m` is 10 MiB/s. |
| `blkio-device-read-iops`, `blkio-device-write-iops` | `path:iops`, an integer. |

- **Separate several devices with commas**, for example `/dev/sda:10m,/dev/sdb:5m`.
- **Set the active values.** Docker keeps an idle limit until another update replaces it.
- **Per-device limits need a daemon with API version 1.55 or newer.** Older daemons accept the update but [ignore them](https://github.com/moby/moby/issues/52650), and Sablier logs a warning. `blkio-weight` works with every version.

If Docker returns warnings for the update, Sablier logs them.

[Learn more about throttling block I/O](/features/disk-io-limits/).

### Readiness

Sablier reports a container:

- **ready** when it is running and its [health check](https://docs.docker.com/reference/compose-file/services/#healthcheck) passes. Without a health check, as soon as it is running.
- **starting** while it is created, paused, restarting or being removed, or running with a health check that is starting or unhealthy.
- **stopped** when it exited with code `0`.
- **completed** when it is a [one-shot container](#one-shot-containers) that finished.
- **error** when it exited with another code, or is dead.

Give your containers a health check. Without one, Sablier logs a warning and lets traffic in before the application may be ready. An unhealthy container counts as still starting, so a broken one fails when the request times out.

### Limitations

- Sablier manages the containers of one Docker daemon. For [Swarm](https://docs.docker.com/engine/swarm/) services, use the [Docker Swarm provider](/providers/docker_swarm/).
- Sablier doesn't create containers: create them before the first request.
- `DOCKER_HOST` doesn't support `ssh://` hosts.
- The strategy applies to every container: you can't pause some and stop others.
- Sablier starts `depends_on` dependencies only for containers that Compose created.
- A start, dependencies included, times out after five minutes.
- Per-device block I/O limits need a daemon with API version 1.55 or newer.

## Examples

- [`examples/docker`](https://github.com/sablierapp/sablier/tree/main/examples/docker): every Compose file on this page, with instructions to run them.
- [Getting started](/tutorials/getting-started/): a complete setup with Docker and Caddy.
