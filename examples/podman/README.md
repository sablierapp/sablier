# Podman Examples

Runnable files for the [Podman provider](https://sablierapp.dev/providers/podman/).
The documentation includes these files directly, so what you read is what you
run. The workloads use [sablierapp/mimic](https://github.com/sablierapp/mimic),
a configurable web-server built for testing purposes.

## Files

| File | Shows |
|---|---|
| `compose.yml` | Sablier with the Podman socket, and the `whoami` container it manages |
| `sablier.sh` | Sablier with `podman run` instead of Compose |
| `whoami.sh` | The `whoami` container with `podman create` instead of Compose |
| `group.yml` | One container in two groups |
| `ready-after.yml` | A settling delay after the health check passes |
| `ready-on-start.yml` | Ready as soon as the container starts |
| `running-hours.yml` | Kept running on weekdays during office hours, with Sablier's time zone |
| `anti-affinity.yml` | Kept stopped while the `streaming` group is active |
| `scale-mode.yml` | Kept running with a lower memory limit instead of stopped (Podman 5.1 or later) |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; Compose and the shell ignore the markers.

## Run it

You need Podman with its API socket enabled, and `podman compose` with a
compose provider (docker-compose or podman-compose).

```bash
# Rootful Podman
sudo systemctl enable --now podman.socket
sudo podman compose up -d

# Rootless Podman
systemctl --user enable --now podman.socket
PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock podman compose up -d
```

Request the `whoami` group through the Sablier API, as a reverse proxy plugin
would. The request opens a one-minute session:

```bash
curl "http://localhost:10000/api/strategies/blocking?group=whoami&session_duration=1m"
```

Sablier starts `whoami` if needed and answers once its health check passes. One minute
after the last request, Sablier stops it again (`podman ps -a` shows it
`Exited`). Run the same `curl` again to wake it up.

To try a setting, add its file to the command:

```bash
podman compose -f compose.yml -f ready-after.yml up -d
```
