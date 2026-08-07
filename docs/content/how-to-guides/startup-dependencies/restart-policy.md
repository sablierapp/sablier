---
title: Honor the restart policy
weight: 170
---

Report a container that exits `0` under a non-restarting policy as `completed`, so a one-shot dependency's `service_completed_successfully` condition can resolve.

```yaml
# compose.yml
services:
  sablier:
    image: sablierapp/sablier:{{< version >}}
    command:
      - start
      - --provider.name=docker
      - --provider.docker.honor-restart-policy=true
      - --strategy.blocking.default-timeout=2m
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  app:
    image: sablierapp/mimic:v0.3.3
    labels:
      - "sablier.enable=true"
      - "sablier.group=demo"
    depends_on:
      init:
        condition: service_completed_successfully

  init:
    image: sablierapp/mimic:v0.3.3
    restart: "no"
```

When `app` is requested, Sablier starts the one-shot `init` container, waits until it reports `completed`, then starts `app`. On later requests `init` is already `completed`, so Sablier does not restart it.

```mermaid
flowchart LR
    req["Request for app"]
    init["init runs<br/>restart: no"]
    exit["init exits 0"]
    completed["Sablier reports completed<br/>honor-restart-policy=true"]
    app["app starts<br/>service_completed_successfully met"]
    req --> init --> exit --> completed --> app
```

By default Sablier reports any container that exits with code `0` as `stopped`. With `--provider.docker.honor-restart-policy=true`, Sablier instead honors the container's [Docker restart policy](https://docs.docker.com/engine/containers/start-containers-automatically/) when it exits successfully:

| Container state | Restart policy | Sablier status |
|---|---|---|
| Exited, code `0` | `no` / `on-failure` | **`completed`**: ran and finished its job |
| Exited, code `0` | `always` / `unless-stopped` | `stopped`: a long-running service that was stopped; started again on demand |
| Exited, non-zero code | any | `error`: surfaced as a failure |

`completed` is distinct from `ready`: a `ready` container is running and serving traffic, whereas a `completed` container ran once and exited. A `completed` dependency satisfies a `service_completed_successfully` condition but never a `service_healthy` one. Docker normalizes an unset restart policy to `no`, so a container with no declared policy behaves like `restart: "no"`.

## When to use it

Use this when a managed service depends on a one-shot init or migration container with `service_completed_successfully`. Without the flag, the exited container is reported as `stopped`, the condition never resolves, and Sablier keeps restarting it on every poll.

## Flags

- [`--provider.docker.honor-restart-policy`](/reference/cli/): report a container that exits `0` under a non-restarting policy as `completed`.

{{< callout type="warning" >}}
`--provider.docker.honor-restart-policy` is **deprecated**. It exists only for backward compatibility and will be removed in v2, where honoring the restart policy becomes the default behavior.
{{< /callout >}}

See the [runnable example](https://github.com/sablierapp/sablier/tree/main/examples/restart-policy).
