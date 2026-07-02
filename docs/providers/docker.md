# Docker

The Docker provider communicates with the `docker.sock` socket to start and stop containers on demand.

## Use the Docker provider

In order to use the docker provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: docker
```

#### **CLI**

```bash
sablier start --provider.name=docker
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_NAME=docker
```

<!-- tabs:end -->

!> **Ensure that Sablier has access to the docker socket!**

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.14.0
    restart: always
    command:
      - start
      - --provider.name=docker
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```
<!-- x-release-please-end -->

## Register containers

For Sablier to work, it needs to know which docker container to start and stop.

You have to register your containers by opting-in with labels.

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    restart: unless-stopped
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

## Strategies

The Docker provider supports two strategies for managing containers:

### Stop Strategy (default)

The `stop` strategy completely stops containers when they become idle and starts them again when needed.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  docker:
    strategy: stop
```

#### **CLI**

```bash
sablier start --provider.docker.strategy=stop
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_DOCKER_STRATEGY=stop
```

<!-- tabs:end -->

### Pause Strategy

The `pause` strategy pauses containers instead of stopping them. This is faster than stop/start as the container state remains in memory, but uses more system resources.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  docker:
    strategy: pause
```

#### **CLI**

```bash
sablier start --provider.docker.strategy=pause
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_DOCKER_STRATEGY=pause
```

<!-- tabs:end -->

## How does Sablier knows when a container is ready?

If the container defines a Healthcheck, then it will check for healthiness before stating the `ready` status.

If the containers do not define a Healthcheck, then as soon as the container has the status `started`, it is considered `ready`.

## Restart policies and one-shot (init) containers

Sablier respects the container's [Docker restart policy](https://docs.docker.com/engine/containers/start-containers-automatically/) when deciding whether an **exited** container is a problem or simply *done*.

This matters for one-shot **init / migration** containers — containers that run a task once, exit with code `0`, and are not meant to run again (for example a database migration that must complete before the application boots).

By default Sablier keeps its **historical behavior** and reports any container that exits with code `0` as `stopped`. To make Sablier honor the restart policy instead, enable the `honor-restart-policy` option (see below). With it enabled, the exited status is decided as follows:

| Container state | Restart policy | Sablier status |
|---|---|---|
| Exited, code `0` | `no` / `on-failure` | **`completed`** — the container ran and finished its job |
| Exited, code `0` | `always` / `unless-stopped` | `starting` — Docker will bring it back, Sablier waits |
| Exited, non-zero code | any | `error` — surfaced as a failure |

> `completed` is a distinct status from `ready`: a `ready` container is running and serving traffic, whereas a `completed` container ran once and exited. A `completed` dependency satisfies a `service_completed_successfully` condition, but never a `service_healthy` one.

> Docker normalizes an **unset** restart policy to `no`, so an unset policy is indistinguishable from an explicit `restart: "no"`. With `honor-restart-policy` enabled, both are reported as `completed`.

### Honor the restart policy

> [!WARNING]
> **Deprecated.** This option only exists to preserve backward compatibility. It
> will be **removed in v2**, where honoring the restart policy becomes the
> default behavior.

When enabled, Sablier honors the container's restart policy on a successful exit
(`no`/`on-failure` → `completed`, `always`/`unless-stopped` → `starting`) instead of
always reporting an exited container as `stopped`. This is required for the
one-shot init / migration container pattern described above (and used by the
[`depends-on`](#ordering-containers-with-depends_on) example).

A complete, runnable example is available in [`examples/restart-policy`](https://github.com/sablierapp/sablier/tree/main/examples/restart-policy).

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  docker:
    honor-restart-policy: true
```

#### **CLI**

```bash
sablier start --provider.docker.honor-restart-policy=true
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_DOCKER_HONOR_RESTART_POLICY=true
```

<!-- tabs:end -->

Default: `false` (historical behavior — exited containers are reported as `stopped`).

## Ordering containers with `depends_on`

When you have a stack where one container must start **after** another — a web app that needs its database to be healthy, or that waits for a migration to finish — declare the relationship with Docker Compose [`depends_on`](https://docs.docker.com/compose/how-tos/startup-order/).

```yaml
services:
  app:
    image: myapp
    restart: unless-stopped
    depends_on:
      db:
        condition: service_healthy
      migration:
        condition: service_completed_successfully
    labels:
      - sablier.enable=true
      - sablier.group=mystack

  db:
    image: postgres
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "pg_isready"]
      interval: 1s
    labels:
      - sablier.enable=true
      - sablier.group=mystack

  migration:
    image: myapp
    command: ["migrate"]
    restart: "no"
    depends_on:
      db:
        condition: service_healthy
```

> Note that `migration` is **not** labeled. It is a one-shot job that exits on
> completion, so there is nothing for Sablier to keep alive or scale to zero —
> it is started on-demand only because `app` declares a `depends_on` on it.

> The `service_completed_successfully` condition relies on the exited
> `migration` container being reported as `completed`. Enable
> [`--provider.docker.honor-restart-policy=true`](#honor-the-restart-policy) so
> Sablier honors the `restart: "no"` policy; without it, the exited container is
> reported as `stopped` and the condition never resolves.

When Sablier is asked to start a container, it reads the `com.docker.compose.depends_on` label (written automatically by `docker compose`) and **starts every dependency first**, recursively, waiting for each declared condition before continuing. All four Compose conditions are supported:

| Condition | Sablier waits until… |
|---|---|
| `service_started` | the dependency is running |
| `service_healthy` | the dependency passes its health check |
| `service_completed_successfully` | the dependency has exited with code `0` (reported as `completed`) |
| `service_running_or_healthy` | the dependency is running, or healthy if it has a health check |

### Do I need `sablier.enable` on a dependency?

This is the key decision when wiring up a multi-service stack:

| You want Sablier to… | Label the dependency? |
|---|---|
| **Start** it on-demand **and stop it** when the group is idle | ✅ `sablier.enable=true` + the same `sablier.group` |
| **Start** it on-demand but **keep it running** (e.g. a database shared by several apps) | ❌ no Sablier labels |
| Run a **one-shot job** (migration / seed / init) that exits by design | ❌ never label it |

- **Starting** a `depends_on` dependency never requires a label. As long as a *labeled* container declares the dependency, Sablier starts the dependency and waits for its condition — even if the dependency has no `sablier.*` labels.
- **Stopping** only ever happens for containers explicitly registered with `sablier.enable=true` in the group. Sablier will never stop a container it was not told to manage.
- **Never** add a one-shot container (one that exits on completion) to a **blocking** group. The blocking strategy waits for every group member to be *running*, so a container that exits by design would prevent the group from ever becoming ready. Leave it unlabeled and let `depends_on` start it instead.

> In short: label a dependency when you want it scaled to zero together with the rest of the group. Leave it unlabeled when it should stay up — Sablier will still start it on-demand to satisfy the ordering.

A complete, runnable example is available in [`examples/depends-on`](https://github.com/sablierapp/sablier/tree/main/examples/depends-on).