# Docker Examples

Runnable Compose files for the [Docker provider](https://sablierapp.dev/providers/docker/).
The documentation includes these files directly, so what you read is what you
run. The workloads use [sablierapp/mimic](https://github.com/sablierapp/mimic),
a configurable web-server built for testing purposes.

## Files

`compose.yml` runs Sablier. Every other file adds to it: pass it after
`compose.yml` with another `-f` flag, and Compose
[merges](https://docs.docker.com/compose/how-tos/multiple-compose-files/merge/)
them into one project named `demo`.

| File | Shows |
|---|---|
| `compose.yml` | Sablier with the Docker socket mounted |
| `whoami.yml` | A container with a health check, the starting point for every other example |
| `group.yml` | One container in two groups |
| `ready-after.yml` | A settling delay after the health check passes |
| `ready-on-start.yml` | Ready as soon as the start is sent |
| `running-hours.yml` | Kept warm on weekdays during office hours |
| `timezone.yml` | The time zone Sablier evaluates running hours in |
| `anti-affinity.yml` | Forced idle while the `streaming` group is active |
| `scale-mode.yml` | Kept running with throttled CPU and memory instead of stopped |
| `blkio.yml` | Kept running with a lower block I/O weight instead of stopped |
| `activity.yml` | Traffic on `whoami`'s port keeping its session open, with eBPF |
| `pause.yml` | Sablier pausing idle containers instead of stopping them |
| `depends-on.yml` | A Compose `depends_on` graph: an app, its database and a one-shot migration |
| `honor-restart-policy.yml` | Sablier reporting the one-shot migration as completed |
| `socket-proxy.yml` | Replaces `compose.yml`: Sablier behind a socket proxy |
| `remote.yml` | Replaces `compose.yml`: Sablier managing a remote daemon over TLS |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; they are plain YAML comments.

## Run it

You need Docker with the Compose plugin and `curl`.

```bash
export COMPOSE_FILE=compose.yml:whoami.yml

docker compose up -d sablier
docker compose create whoami
```

`create` makes the container without starting it: Sablier starts it on the
first request. Request the `whoami` group through the Sablier API, as a reverse
proxy plugin would. The request opens a one-minute session:

```bash
curl "http://localhost:10000/api/strategies/blocking?group=whoami&session_duration=1m"
```

Sablier starts `demo-whoami-1` and answers once its health check passes. One
minute after the last request, Sablier stops it (`docker compose ps -a` shows
it `Exited`). Run the same `curl` again to start it back.

Add any other file to `COMPOSE_FILE` the same way to try that setting, then
create its services. Two exceptions:

- `scale-mode.yml` and `blkio.yml` keep the container running: start it with
  `docker compose up -d` instead of `create`.
- `depends-on.yml` needs `honor-restart-policy.yml`:

  ```bash
  export COMPOSE_FILE=compose.yml:honor-restart-policy.yml:depends-on.yml
  docker compose up -d sablier
  docker compose create app db migration
  curl "http://localhost:10000/api/strategies/blocking?group=app&session_duration=1m"
  ```

`activity.yml` needs a Linux host on amd64 or arm64 with cgroup v2, and a
Sablier release with activity tracking. [`examples/activity`](../activity) has
a runnable demo.

`socket-proxy.yml` and `remote.yml` replace `compose.yml`, for example
`COMPOSE_FILE=socket-proxy.yml:whoami.yml`. `remote.yml` needs a reachable
daemon and its client certificates in `./certs`.

Clean up with `docker compose down`.
