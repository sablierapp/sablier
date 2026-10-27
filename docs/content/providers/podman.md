---
title: Podman
weight: 4
since: v1.10.0
aliases:
  - /tutorials/providers/podman/
---

## Introduction

The Podman provider stops [containers](https://docs.podman.io/en/latest/markdown/podman-container.1.html) when their session expires, and starts them again when a request needs them. Sablier calls the Docker-compatible API of the [Podman API service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html) through the Podman socket. It runs the code of the [Docker provider](/providers/docker/), with the differences listed on this page.

## Install Sablier

Enable the Podman socket first. Sablier only sees the containers of the Podman service it connects to:

```bash
# Rootful containers: /run/podman/podman.sock
sudo systemctl enable --now podman.socket

# Your rootless containers: $XDG_RUNTIME_DIR/podman/podman.sock
systemctl --user enable --now podman.socket
```

[Socket activation](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html#run-the-command-in-a-systemd-service) starts the API service when Sablier connects. See [rootless mode](https://docs.podman.io/en/latest/markdown/podman.1.html#rootless-mode) for the difference between the two sockets.

Then run Sablier in a container, with the socket mounted:

{{< tabs >}}
{{< tab name="Compose" >}}

Add Sablier to your [Compose file](https://docs.podman.io/en/latest/markdown/podman-compose.1.html):

{{< example file="podman/compose.yml" region="sablier" >}}

For rootless Podman, set `PODMAN_SOCKET` to your user's socket:

```bash
PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock podman compose up -d
```

Reverse proxy plugins in the same Compose project reach Sablier at `http://sablier:10000`.

{{< /tab >}}
{{< tab name="podman run" >}}

Run Sablier with [`podman run`](https://docs.podman.io/en/latest/markdown/podman-run.1.html):

{{< example file="podman/sablier.sh" region="run" >}}

For rootless Podman, set `PODMAN_SOCKET` to your user's socket:

```bash
PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock ./sablier.sh
```

Reverse proxy plugins reach Sablier on port `10000` of the host, or at `http://sablier:10000` from a container on the same [network](https://docs.podman.io/en/latest/markdown/podman-network.1.html).

{{< /tab >}}
{{< /tabs >}}

- **Mount the socket at `/run/podman/podman.sock`.** It is the default [address](#configuration), whatever the path on the host.
- **Disable SELinux separation for Sablier.** On SELinux hosts, a container can only use the socket [with `label=disable`](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html#access-the-unix-socket-from-inside-a-container).

{{< callout type="warning" >}}
The [Podman API](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html) allows arbitrary code execution as the user that runs it: root, for rootful Podman. Only give the socket to Sablier.
{{< /callout >}}

## Usage example

Opt a container in with the `sablier.enable` [label](https://docs.podman.io/en/latest/markdown/podman-run.1.html#label-l-key-value), and give it a group:

{{< example file="podman/compose.yml" region="whoami" >}}

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of the `whoami` [container](https://docs.podman.io/en/latest/markdown/podman-container.1.html). On the first request, Sablier starts the container and holds the request, or shows a waiting page, until its [health check](https://docs.podman.io/en/latest/markdown/podman-run.1.html#health-cmd-command-command-arg1) passes. When the session expires, Sablier stops it.

{{< callout type="info" >}}
Sablier stops opted-in containers when it starts. A container you start afterwards keeps running until its first session expires, unless you [stop externally-started instances](/features/stop-external-starts/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.podman.uri`](/reference/cli/#opt-provider-podman-uri) | `unix:///run/podman/podman.sock` | The address of the [Podman API service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html): a `unix://` socket or a `tcp://` address. |

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/). The `--provider.docker.*` options do not apply.

With [tracing](/features/tracing/) enabled, every call to the Podman API is a child span of the request that triggered it.

{{< details title="Other addresses" closed="true" >}}
Sablier connects to `--provider.podman.uri` only: it ignores the `CONTAINER_HOST` and `DOCKER_HOST` environment variables. `ssh://` addresses are not supported.
{{< /details >}}

### Workloads

#### Containers

Sablier manages the [containers](https://docs.podman.io/en/latest/markdown/podman-container.1.html) that carry `sablier.enable=true`, running or not:

- **Stop** works like [`podman stop`](https://docs.podman.io/en/latest/markdown/podman-stop.1.html): the container gets its stop signal, and is killed after its stop timeout. Sablier waits until it has exited.
- **Start** works like [`podman start`](https://docs.podman.io/en/latest/markdown/podman-start.1.html). Sablier leaves a running container as is.

Create the containers with [`podman compose`](https://docs.podman.io/en/latest/markdown/podman-compose.1.html), as in the usage example, or with [`podman create`](https://docs.podman.io/en/latest/markdown/podman-create.1.html):

{{< example file="podman/whoami.sh" region="run" >}}

Sablier watches [Podman events](https://docs.podman.io/en/latest/markdown/podman-events.1.html): a container you create joins its groups without restarting Sablier.

For containers that [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) runs as systemd services, use the [systemd provider](/providers/systemd/) instead: it starts and stops the unit.

### Instance names

A reverse proxy can target one container by name instead of a group. The instance name is the [container name](https://docs.podman.io/en/latest/markdown/podman-run.1.html#name-name), as [`podman ps`](https://docs.podman.io/en/latest/markdown/podman-ps.1.html) shows it.

Compose builds the name from the project and the service. Run from the `examples/podman` folder, the `whoami` container of the usage example is named:

```text
podman_whoami_1
```

That is the [podman-compose](https://github.com/containers/podman-compose) form; Docker Compose names it `podman-whoami-1`. Set [`container_name`](https://docs.docker.com/reference/compose-file/services/#container_name) to choose the name yourself.

Prefer groups: a name ties your proxy configuration to the tool that created the container.

### Labels

Sablier settings are [container labels](https://docs.podman.io/en/latest/markdown/podman-run.1.html#label-l-key-value) that start with `sablier.`:

{{< example file="podman/group.yml" region="sablier" >}}

- **Set labels when you create the container.** Podman can't change the labels of an existing container, so recreate it, for example with `podman compose up -d`.
- **`sablier.enable` must be exactly `true`.** Sablier lists containers with that label and value.
- **Quote `true` and numbers in a Compose map.** [Compose labels](https://docs.docker.com/reference/compose-file/services/#labels) are strings.

With `podman create` or `podman run`, pass each label with `--label`, as in [`whoami.sh`](#containers). The [label reference](/reference/labels/) lists every key.

### Workload settings

Each excerpt below is the `labels` of a complete Compose file in [`examples/podman`](https://github.com/sablierapp/sablier/tree/main/examples/podman). Run one next to Sablier with `podman compose -f compose.yml -f <file> up -d`.

#### Enable {since="v1.10.0"}

`sablier.enable=true` opts the container in. It is required, and any other value is ignored. See the [usage example](#usage-example).

#### Group {since="v1.10.0"}

`sablier.group` puts the container in one or more groups, separated by commas. A session for any of them starts it. Without it, the container is in the `default` group.

{{< example file="podman/group.yml" region="sablier" >}}

A list of groups needs v1.13.0 or later. [Learn more about groups](/features/groups/).

#### Ready after {since="v1.13.0"}

`sablier.ready-after` keeps reporting the container as not ready for a settling delay after it turns healthy. Use it when a container passes its [health check](https://docs.podman.io/en/latest/markdown/podman-run.1.html#health-cmd-command-command-arg1) before it can serve traffic.

{{< example file="podman/ready-after.yml" region="sablier" >}}

[Learn more about delaying readiness](/features/ready-after/).

#### Ready on start {since="v1.15.0"}

`sablier.ready-on-start=true` reports the container ready as soon as Sablier starts it, without waiting for its [health check](https://docs.podman.io/en/latest/markdown/podman-run.1.html#health-cmd-command-command-arg1).

{{< example file="podman/ready-on-start.yml" region="sablier" >}}

[Learn more about marking ready on start](/features/ready-on-start/).

#### Running hours {since="v1.13.0"}

- `sablier.running-hours` keeps the container running during a daily window.
- `sablier.running-days` limits that window to some weekdays, since v1.15.0.

{{< example file="podman/running-hours.yml" region="sablier" >}}

The window uses Sablier's local time, which is UTC in the Sablier image. Set the `TZ` [environment variable](https://docs.podman.io/en/latest/markdown/podman-run.1.html#env-e-env) of the Sablier container to change it:

{{< example file="podman/running-hours.yml" region="timezone" >}}

[Learn more about running hours](/features/running-hours/).

#### Anti-affinity {since="v1.15.0"}

`sablier.anti-affinity` lists groups the container backs off from: Sablier keeps it idle while any of them has an active session.

{{< example file="podman/anti-affinity.yml" region="sablier" >}}

[Learn more about anti-affinity](/features/anti-affinity/).

#### Scale instead of stop {since="v1.13.0"}

Keep the container running when idle, optionally with lower CPU and memory limits, instead of stopping it:

- `sablier.idle.replicas=1` keeps the container running when idle.
- `sablier.idle.cpu` and `sablier.idle.memory` set its [CPU](https://docs.podman.io/en/latest/markdown/podman-update.1.html#cpu-quota-limit) and [memory](https://docs.podman.io/en/latest/markdown/podman-update.1.html#memory-m-number-unit) limits while idle. They need `sablier.idle.replicas=1`.
- `sablier.active.cpu` and `sablier.active.memory` set the limits again on the next session.

CPU values are cores, such as `0.5`. Sablier sends them as a CPU quota over a 100 ms period, the form Podman's Docker-compatible API reads.

Memory values use the suffixes `b`, `k`, `m` and `g`, such as `64m` or `1g`. Sablier sets the swap limit to the same value, which disables swap for the container.

{{< example file="podman/scale-mode.yml" region="sablier" >}}

- **Memory limits need Podman 5.1 or later.** Older versions lack the Docker-compatible update endpoint that Sablier calls.
- **Rootless Podman may refuse to change limits.** It depends on the host's cgroup setup; see [`podman update`](https://docs.podman.io/en/latest/markdown/podman-update.1.html#cpu-period-limit).

[Learn more about scaling instead of stopping](/features/scale-mode/).

### Readiness

Sablier reports a container:

- **ready** when it is running and its [health check](https://docs.podman.io/en/latest/markdown/podman-run.1.html#health-cmd-command-command-arg1) passes. A running container without a health check is ready at once, and Sablier logs a warning.
- **starting** when it is created, restarting or being removed, or running with a health check that has not passed yet. An unhealthy container counts as starting: Sablier keeps waiting until the request times out.
- **stopped** when it exited with code `0`, or is paused.
- **error** when it exited with another code, or is dead.

Give your containers a health check, so Sablier sends traffic only once they can serve it.

{{< callout type="warning" >}}
Podman schedules health checks with systemd timers. On a host without systemd, they never run, and a container with a health check stays starting. Run them yourself with [`podman healthcheck run`](https://docs.podman.io/en/latest/markdown/podman-healthcheck-run.1.html), or remove the health check.
{{< /callout >}}

### Limitations

- **Sablier always stops containers.** The pause strategy, [`--provider.docker.strategy`](/reference/cli/#opt-provider-docker-strategy), is Docker-only.
- **The restart policy is not honored.** [`--provider.docker.honor-restart-policy`](/reference/cli/#opt-provider-docker-honor-restart-policy) is Docker-only: a container that exits with code `0` is reported stopped, never completed.
- **podman-compose does not record `depends_on`.** Sablier reads [`depends_on`](https://docs.docker.com/reference/compose-file/services/#depends_on) from the `com.docker.compose.depends_on` label, which Docker Compose writes and [podman-compose](https://github.com/containers/podman-compose) does not. Put the dependencies in the same group instead. See [starting dependencies together](/providers/docker/#start-dependencies-together).
- [Block I/O throttling](/features/disk-io-limits/) is not available on Podman.

## Examples

- [`examples/podman`](https://github.com/sablierapp/sablier/tree/main/examples/podman): every file on this page, with instructions to run them.
