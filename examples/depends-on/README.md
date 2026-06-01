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

The long-running services (`app` and `db`) carry the same `sablier.group=app`
and `sablier.enable=true` labels, so Sablier scales them to zero when idle and
brings them back up — in dependency order — on the next request. The
`migration` container is a **one-shot init job**: it is intentionally left
**unlabeled** because it is not something Sablier should keep alive or scale to
zero — it simply runs once whenever `app` starts. See
["Should I label dependencies?"](#should-i-label-dependencies) below for the full
reasoning.

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
and has `restart: "no"`. With `--provider.docker.honor-restart-policy=true`
(set on the `sablier` service in this example), Sablier honors the restart
policy — a container that exited successfully under a non-restarting policy is
reported as **ready (completed)** instead of "stopped". This is what allows the
`service_completed_successfully` condition to resolve.

> The `honor-restart-policy` option is **deprecated** and only exists for
> backward compatibility. It will become the default in v2. Without it, Sablier
> keeps its historical behavior and reports the exited `migration` container as
> "stopped".

See [#792](https://github.com/sablierapp/sablier/issues/792) (depends_on) and
[#952](https://github.com/sablierapp/sablier/issues/952) (restart policy /
init containers).

## Should I label dependencies?

Whether you add `sablier.enable=true` to a dependency depends on whether you
want Sablier to **stop** it:

| You want Sablier to… | Label the dependency? |
|---|---|
| Start it on-demand **and** scale it to zero when idle (long-running service) | ✅ `sablier.enable=true` + `sablier.group=<group>` |
| Start it on-demand but keep it running 24/7 (e.g. a shared database) | ❌ no Sablier labels |
| Run a one-shot job (migration, seed, init container) that exits by design | ❌ **never** label it |

Key points:

- **Starting** a `depends_on` dependency does **not** require any Sablier
  labels. As long as a labeled container declares the dependency, Sablier will
  start it (and wait for its condition) — even if the dependency itself has no
  `sablier.*` labels.
- **Stopping** a container only happens for containers that are explicitly
  registered with `sablier.enable=true` and belong to the group. Sablier never
  stops a container it was not told to manage.
- **Never** put a one-shot container (one that exits on completion, like a
  migration) into a **blocking** group. The blocking strategy waits for every
  group member to be *running*; a container that exits by design would keep the
  group from ever becoming ready. Leave it unlabeled and let `depends_on`
  trigger it instead — that is exactly what `migration` does here.

This example labels `db` so it is scaled to zero together with `app`. If `db`
were a database shared by several apps that you don't want to shut down, you
would simply drop its `sablier.*` labels — Sablier would still start it
on-demand to satisfy the `depends_on`, but would leave it running. `migration`
is left unlabeled because it is a one-shot job.

## Services

| Service | Managed by Sablier | Role |
|---|---|---|
| `sablier` | — | Manages the `app` group; exposes the REST API on `:10000` |
| `app` | ✅ `sablier.group=app` | The application; depends on `db` (healthy) and `migration` (completed) |
| `db` | ✅ `sablier.group=app` | `sablierapp/mimic` with a health check; resolves `service_healthy`. Started before its dependents and stopped with the group |
| `migration` | ❌ unlabeled | One-shot init container (`restart: "no"`, exits `0`); started on-demand via `app`'s `depends_on` and resolves `service_completed_successfully` |

## Prerequisites

- Docker with Compose plugin (`docker compose version`)
- `curl` and `jq` for the walkthrough

## Walkthrough

### 1. Start Sablier and create the dependency graph

```bash
make up
```

This starts only `sablier`, then **creates** `app`, `migration` and `db`
without starting them, so the whole chain is scaled to zero and ready to be
triggered on-demand. (The managed containers are deliberately not started:
because they carry `sablier.enable=true`, Sablier would otherwise immediately
stop them again.)

### 2. Confirm everything is stopped

```bash
make ps
```

You should see `app`, `db` and `migration` in a `Created` state and only
`sablier` running.

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

### 5. Watch the stack scale back to zero

Stop sending requests and wait for the session to expire (the stack uses
`--sessions.default-duration=2m`). Because `db` carries the
`sablier.enable`/`sablier.group=app` labels, Sablier stops **both** `app` and
`db` together (the `migration` container already exited on its own after
completing):

```bash
make ps   # after ~2 minutes: app and db are stopped again; migration is Exited(0)
```

> If `db` did not carry the Sablier labels, it would be started on-demand but
> would **stay running** after the session expires. Labeling it is what allows
> the database to be scaled to zero with the rest of the group.

### 6. Tear down

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
