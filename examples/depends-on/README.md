# `depends_on` Example

This example demonstrates how Sablier's Docker provider resolves Docker Compose
[`depends_on`](https://docs.docker.com/compose/how-tos/startup-order/)
relationships and respects container **restart policies**, using
[sablierapp/mimic](https://github.com/sablierapp/mimic), a configurable
web-server built for testing purposes.

Many applications cannot start in isolation. A typical web app needs its
**database** to be healthy and its **schema migration** to have completed
*before* it can serve traffic. When Sablier scales such an app down to zero, it
must bring the whole dependency graph back up — in the right order — the moment
a request arrives.

This example reproduces that exact scenario:

```
Dependency graph (declared in compose.yml):

  app ──depends_on──▶ db        (condition: service_healthy)
   │
   └────depends_on──▶ migration  (condition: service_completed_successfully)
                          │
                          └─depends_on──▶ db (condition: service_healthy)
```

Only `app` carries the `sablier.enable`/`sablier.group` labels. `db` and
`migration` are **not** managed by Sablier directly — they are pulled in
automatically as dependencies.

## How it works

When a blocking request for the `app` group arrives, the Docker provider:

1. Inspects `app` and reads its `com.docker.compose.depends_on` label
   (Compose generates this automatically from the `depends_on` block).
2. Starts each dependency **before** `app`, recursively resolving their own
   `depends_on` graphs:
   - `db` is started and awaited until its health check passes
     (`service_healthy`).
   - `migration` is started and awaited until it exits with code `0`
     (`service_completed_successfully`).
3. Starts `app` only once every dependency condition is satisfied.

### Why the migration container matters

`migration` is a one-shot init container: it runs once, exits with code `0`,
and has `restart: "no"`. Sablier now respects the restart policy — a container
that exited successfully under a non-restarting policy is reported as **ready
(completed)** instead of being seen as "stopped" and restarted forever. This is
what allows the `service_completed_successfully` condition to resolve.

See [#792](https://github.com/sablierapp/sablier/issues/792) (depends_on) and
[#952](https://github.com/sablierapp/sablier/issues/952) (restart policy /
init containers).

## Services

| Service | Managed by Sablier | Role |
|---|---|---|
| `sablier` | — | Manages the `app` group; exposes the REST API on `:10000` |
| `app` | ✅ `sablier.group=app` | The application; depends on `db` (healthy) and `migration` (completed) |
| `db` | ➖ dependency | `sablierapp/mimic` with a health check; resolves `service_healthy` |
| `migration` | ➖ dependency | One-shot init container (`restart: "no"`, exits `0`); resolves `service_completed_successfully` |

## Prerequisites

- Docker with Compose plugin (`docker compose version`)
- `curl` and `jq` for the walkthrough

## Walkthrough

### 1. Start Sablier and stop the dependency graph

```bash
make up
```

This starts everything, then stops `app`, `migration` and `db` so the entire
chain is scaled to zero and ready to be triggered on-demand.

### 2. Confirm everything is stopped

```bash
make ps
```

You should see `app`, `db` and `migration` in an `Exited`/`Created` state and
only `sablier` running.

### 3. Watch Sablier logs in a separate terminal

```bash
make logs
```

### 4. Send a blocking request for the `app` group

```bash
make start
```

Sablier will resolve the dependency graph before returning:

1. Start `db`, poll until its health check passes.
2. Start `migration`, poll until it exits successfully.
3. Start `app`, poll until it is healthy.
4. Return the JSON response to `curl`.

### 5. Tear down

```bash
make down
```

## What to look for in the logs

```
level=DEBUG msg="starting depends_on dependency" dependency=db        condition=service_healthy
level=DEBUG msg="starting depends_on dependency" dependency=migration condition=service_completed_successfully
level=DEBUG msg="starting container" name=db
level=DEBUG msg="starting container" name=migration
level=DEBUG msg="starting container" name=app
```

The blocking request holds open until `db` is healthy and `migration` has
completed, then `app` is started and the request resolves.
