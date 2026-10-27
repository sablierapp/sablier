# Systemd Examples

Runnable unit files for the [systemd provider](https://sablierapp.dev/providers/systemd/).
The documentation includes these files directly, so what you read is what you
install. Every example unit runs `sleep infinity`: a stand-in for your own
service, to show the start and stop lifecycle.

## Files

| File | Shows |
|---|---|
| `sablier.yaml` | Sablier configuration for the systemd **user** instance |
| `sablier.service` | User unit that runs Sablier (`systemctl --user`) |
| `system/sablier.yaml` | Sablier configuration for the systemd **system** instance |
| `system/sablier.service` | System unit that runs Sablier as root |
| `demo.service` | A unit in the `demo` group, the starting point for every other example |
| `group.service` | One unit in two groups |
| `ready-after.service` | A settling delay after the unit is active |
| `ready-on-start.service` | Ready as soon as the start job is done |
| `running-hours.service` | Kept running on weekdays during office hours |
| `anti-affinity.service` | Kept idle while the `streaming` group is active |
| `scale-mode.service` | Kept running with lower CPU and memory limits instead of stopped |
| `blkio.service` | Kept running with block I/O throttled instead of stopped |
| `Makefile` | `download` / `install` / `add` / `uninstall` / `start` / `status` / `logs` |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; systemd and Sablier read them as comments.

## Run it

The `Makefile` uses the systemd **user** instance, so you need no root access.
You need a Linux host with systemd, a running user session, `curl` and `jq`.

```bash
make download   # Sablier release binary to ~/.local/bin/sablier
make install    # sablier.yaml, sablier.service and demo.service
```

`make install` copies the files to `~/.config/sablier` and
`~/.config/systemd/user`, reloads the user manager, enables both units and
starts Sablier. It only enables `demo.service`: an enabled unit stays loaded, so
Sablier finds it, but it does not run until a session requests it.

Request the `demo` group through the Sablier API, as a reverse proxy plugin
would. The request opens a one-minute session:

```bash
make start      # curl "http://localhost:10000/api/strategies/blocking?group=demo&session_duration=1m"
make status     # demo.service is active
```

One minute after the last request, Sablier stops `demo.service`
(`make status` shows it inactive). Run `make start` again: Sablier starts it
and answers once systemd reports it active.

Install any other example unit to try that setting, then request its group:

```bash
make add UNIT=ready-after.service
curl "http://localhost:10000/api/strategies/blocking?group=ready-after&session_duration=1m"
```

`make logs` follows Sablier's logs. `make uninstall` removes Sablier and every
example unit.

## System instance

To manage system units instead, install the files from `system/` as root:

```bash
sudo install -D -m 0644 system/sablier.yaml /etc/sablier/sablier.yaml
sudo install -m 0644 system/sablier.service /etc/systemd/system/sablier.service
sudo install -m 0644 demo.service /etc/systemd/system/demo.service
sudo systemctl daemon-reload
sudo systemctl enable demo.service
sudo systemctl enable --now sablier.service
```

This expects the Sablier binary at `/usr/local/bin/sablier`.
