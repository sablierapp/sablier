# Docker Swarm Examples

Runnable stack files for the [Docker Swarm provider](https://sablierapp.dev/providers/docker_swarm/).
The documentation includes these files directly, so what you read is what you
deploy. The workloads use [sablierapp/mimic](https://github.com/sablierapp/mimic),
a configurable web-server built for testing purposes.

## Files

| File | Shows |
|---|---|
| `sablier.yml` | Sablier as a Swarm service on a manager node, with the `sablier` overlay network |
| `whoami.yml` | A service with a healthcheck, the starting point for every other example |
| `replicas.yml` | Three replicas on start, zero when idle |
| `group.yml` | One service in two groups |
| `ready-after.yml` | A settling delay after the tasks are healthy |
| `ready-on-start.yml` | Ready as soon as the scale-up is sent |
| `running-hours.yml` | Kept running on weekdays during office hours |
| `anti-affinity.yml` | Kept idle while the `streaming` group is active |
| `scale-mode.yml` | Kept at one throttled replica instead of zero |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; Docker ignores the comments.

## Run it

You need a Docker Engine in [Swarm mode](https://docs.docker.com/engine/swarm/).
Run the commands on a manager node.

```bash
docker swarm init # once, if the node is not in a swarm yet

docker stack deploy -c whoami.yml demo
docker stack deploy -c sablier.yml sablier
```

Sablier scales the opted-in `demo_whoami` service to zero when it starts.

Request the `whoami` group through the Sablier API, as a reverse proxy plugin
would. The request opens a one-minute session:

```bash
docker run --rm --network sablier curlimages/curl:8.11.1 \
  "http://sablier:10000/api/strategies/blocking?group=whoami&session_duration=1m"
```

Sablier scales `demo_whoami` to one replica and answers once its task is
healthy. One minute after the last request, Sablier scales it back to zero
(`docker service ls` shows `0/0`).

Deploy any other file the same way to try that setting, for example
`docker stack deploy -c group.yml demo`. A service deployed after Sablier
started keeps running until its first session expires.

Clean up with:

```bash
docker stack rm demo sablier
```
