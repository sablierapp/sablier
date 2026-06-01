# Restart Policy Example

This example demonstrates how Sablier's Docker provider reports the status of an
**exited** container based on its [Docker restart
policy](https://docs.docker.com/engine/containers/start-containers-automatically/),
using [sablierapp/mimic](https://github.com/sablierapp/mimic), a configurable
web-server built for testing purposes.

By default, Sablier keeps its **historical behavior** and reports any container
that exits with code `0` as `stopped`. When you enable
`--provider.docker.honor-restart-policy=true`, Sablier instead honors the
restart policy when a container exits successfully:

| Container state | Restart policy | Sablier status |
|---|---|---|
| Exited, code `0` | `no` / `on-failure` | **`ready`** — the container completed its job |
| Exited, code `0` | `always` / `unless-stopped` | `starting` — Docker will bring it back, Sablier waits |
| Exited, non-zero code | any | `error` — surfaced as a failure |

> [!WARNING]
> `--provider.docker.honor-restart-policy` is **deprecated**. It only exists to
> preserve backward compatibility and will be **removed in v2**, where honoring
> the restart policy becomes the default behavior.

> Docker normalizes an **unset** restart policy to `no`, so a container with no
> declared policy is reported exactly like one with an explicit `restart: "no"`.

## What this example shows

The `demo` group contains two members that share `sablier.group=demo` and
`sablier.enable=true`:

- **`app`** — a long-running web service (`restart: unless-stopped`). While
  running it is reported as `ready`.
- **`init`** — a one-shot init container (`restart: "no"`) that runs once,
  exits with code `0`, and is never restarted.

Normally you must **never** put a one-shot container in a blocking group: the
blocking strategy waits for every member to be running, so a container that
exits by design would block the group forever.

With `--provider.docker.honor-restart-policy=true`, Sablier honors the `"no"`
restart policy and reports the completed `init` container as **`ready`**
(completed) instead of `stopped`. That lets the blocking `demo` group reach the
ready state even though one of its members is a one-shot init job.

## How it works

1. `make up` starts Sablier and *creates* (scaled to zero) the `app` and `init`
   containers.
2. `make start` issues a blocking request for the `demo` group.
3. Sablier starts both containers and waits until the group is ready:
   - `app` reaches the `ready` state once it is running and healthy.
   - `init` runs, exits with code `0`, and — thanks to
     `honor-restart-policy` — is reported as `ready` (completed).
4. The blocking request returns once both members are ready.

## Try it

```bash
make up      # start Sablier and create the group (scaled to zero)
make start   # blocking request — returns once the group is ready
make logs    # follow Sablier logs
make ps      # inspect container states
make down    # tear everything down
```

To see the historical behavior, remove
`--provider.docker.honor-restart-policy=true` from the `sablier` command in
[`compose.yml`](./compose.yml): the exited `init` container is then reported as
`stopped` and the blocking group never becomes ready.

See [#952](https://github.com/sablierapp/sablier/issues/952) for the original
discussion about restart policies and init containers.
