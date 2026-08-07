# Restart Policy Example

This example demonstrates how Sablier's Docker provider reports the status of an
**exited** container based on its [Docker restart
policy](https://docs.docker.com/engine/containers/start-containers-automatically/),
and why that matters for one-shot **init / migration** containers, using
[sablierapp/mimic](https://github.com/sablierapp/mimic), a configurable
web-server built for testing purposes.

By default, Sablier keeps its **historical behavior** and reports any container
that exits with code `0` as `stopped`. When you enable
`--provider.docker.honor-restart-policy=true`, Sablier instead honors the
restart policy when a container exits successfully:

| Container state | Restart policy | Sablier status |
|---|---|---|
| Exited, code `0` | `no` / `on-failure` | **`completed`** — the container ran and finished its job |
| Exited, code `0` | `always` / `unless-stopped` | `stopped` — a long-running service that was stopped; Sablier starts it again on demand |
| Exited, non-zero code | any | `error` — surfaced as a failure |

> `completed` is a distinct status from `ready`: a `ready` container is running
> and serving traffic, whereas a `completed` container ran once and exited. A
> `completed` dependency satisfies a `service_completed_successfully` condition,
> but never a `service_healthy` one.

> [!WARNING]
> `--provider.docker.honor-restart-policy` is **deprecated**. It only exists to
> preserve backward compatibility and will be **removed in v2**, where honoring
> the restart policy becomes the default behavior.

> Docker normalizes an **unset** restart policy to `no`, so a container with no
> declared policy is reported exactly like one with an explicit `restart: "no"`.

## What this example shows

Two containers:

- **`app`** — a long-running web service (`restart: unless-stopped`), labeled
  with `sablier.enable=true` and `sablier.group=demo`. It declares a
  `depends_on` on `init` with condition `service_completed_successfully`.
- **`init`** — a one-shot init container (`restart: "no"`) that runs once, exits
  with code `0`, and is never restarted. It is **not** labeled; Sablier starts
  it on demand only to satisfy `app`'s `depends_on`.

When Sablier is asked to start `app`, it first starts `init` and waits until it
has **completed successfully** before starting `app`. This only works because
`honor-restart-policy` makes the exited `init` container report as `completed`:

- **With** `honor-restart-policy=true`: `init` exits `0` → reported `completed`
  → `service_completed_successfully` resolves → `app` starts. On subsequent
  requests, `init` is already `completed`, so Sablier does **not** restart it.
- **Without** the flag: `init` exits `0` → reported `stopped` → the condition
  never resolves and Sablier keeps restarting `init` on every poll
  (the bug described in [#952](https://github.com/sablierapp/sablier/issues/952)).

> A one-shot container must **never** be a labeled member of a **blocking**
> group: the blocking strategy waits for every member to be `ready` (running),
> and a `completed` container is not `ready`, so the group would never resolve.
> Keep the one-shot unlabeled and let `depends_on` start it, as shown here.

## How it works

1. `make up` starts Sablier and *creates* (scaled to zero) the `app` and `init`
   containers.
2. `make start` issues a blocking request for the `demo` group.
3. Sablier resolves `app`'s `depends_on`: it starts `init`, waits until it has
   completed (exit `0` → `completed`), then starts `app` and waits until it is
   `ready`.
4. The blocking request returns once `app` is ready.

## Try it

```bash
make up      # start Sablier and create the containers (scaled to zero)
make start   # blocking request — returns once app is ready
make logs    # follow Sablier logs
make ps      # inspect container states
make down    # tear everything down
```

To see the historical behavior, remove
`--provider.docker.honor-restart-policy=true` from the `sablier` command in
[`compose.yml`](./compose.yml): the exited `init` container is then reported as
`stopped`, its `service_completed_successfully` condition never resolves, and
the blocking request times out.

See [#952](https://github.com/sablierapp/sablier/issues/952) for the original
discussion about restart policies and init containers.
