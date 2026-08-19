# Systemd Provider Example (user instance)

This example shows how to run Sablier with the **systemd provider** using the
**user** D-Bus. Instead of managing Docker
containers, Sablier starts and stops systemd **user** units that opt in via an
`[X-Sablier]` section in their unit file.

The demo unit just runs `sleep infinity`; it is a stand-in to showcase the
start/stop lifecycle.

```
┌────────────────────────────────────────────────────────────────────────┐
│ User session (systemd --user)                                          │
│                                                                        │
│   ┌──────────┐                                                         │
│   │  curl    │                                                         │
│   └────┬─────┘                                                         │
│        │ blocking request (group=demo)                                 │
│        ▼                                                               │
│   ┌────────────┐                                                       │
│   │  sablier   │                                                       │
│   │ (systemd   │                                                       │
│   │  provider) │                                                       │
│   └─────┬──────┘                                                       │
│         │ D-Bus                                                        │
│         ▼                                                              │
│   ┌────────────────────┐                                               │
│   │  demo.service      │                                               │
│   │  [X-Sablier]       │                                               │
│   └────────────────────┘                                               │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

When a session is requested, Sablier asks systemd to **start** `demo.service`;
when the session expires, it asks systemd to **stop** it again. Sablier considers the unit ready as soon as systemd reports it as `active`.

## Prerequisites

- A Linux host with systemd and a running user session
  works.
- The `sablier` binary installed at `~/.local/bin/sablier`.
- `curl` and `jq` for the walkthrough

## Files

| File              | Purpose                                                                |
| ----------------- | ---------------------------------------------------------------------- |
| `sablier.yaml`    | Sablier configuration — selects the `systemd` provider (user instance) |
| `demo.service`    | The demo unit Sablier manages (carries `[X-Sablier]`)                  |
| `sablier.service` | Unit to run Sablier itself via `systemd --user`                        |
| `Makefile`        | `install` / `uninstall` / `start` / `status` / `logs` / `demo`         |

## Sablier settings vs. `[X-Sablier]` keys

systemd units have no labels, so Sablier reads an `[X-Sablier]` section from the
unit file instead. The keys use systemd-style casing and drop the `sablier.`
prefix:

| `sablier.*` label         | `[X-Sablier]` key |
| ------------------------- | ----------------- |
| `sablier.enable`          | `Enable`          |
| `sablier.group`           | `Group`           |
| `sablier.ready-after`     | `ReadyAfter`      |
| `sablier.idle.replicas`   | `IdleReplicas`    |
| `sablier.idle.cpu`        | `IdleCPU`         |
| `sablier.idle.memory`     | `IdleMemory`      |
| `sablier.active.replicas` | `ActiveReplicas`  |
| `sablier.active.cpu`      | `ActiveCPU`       |
| `sablier.active.memory`   | `ActiveMemory`    |

`Enable=true` is required — Sablier refuses to start or stop any unit that has not explicitly opted in.

## Walkthrough

### 1. Install and start everything

```bash
make install
```

This copies `sablier.yaml` and the two unit files into `~/.config/systemd/user`,
runs `systemctl --user daemon-reload`, enables both units, and starts
`sablier.service`. The `demo.service` unit is **enabled but not started**. It stays stopped until it is requested.

### 2. Request a session (blocking strategy)

```bash
make start
```

Sablier asks systemd to start `demo.service` and returns once it is `active`:

```json
{
  "session": {
    "instances": [
      {
        "instance": {
          "name": "demo.service",
          "currentReplicas": 1,
          "desiredReplicas": 1,
          "status": "ready",
          "groups": ["demo"],
          "enabled": "true",
          "provider": "systemd",
          "readyAt": "2026-08-19T20:51:41.376278224+02:00",
          "config": {
            "enabled": true,
            "groups": ["demo"]
          }
        }
      }
    ],
    "status": "ready"
  }
}
```

### 3. Confirm the unit is running

```bash
make status
```

### 4. Watch it stop on expiry

The session lasts 1 minute. Wait for it to expire, then check again:

```bash
sleep 70
make status
```

### 5. Tear down

```bash
make uninstall
```

## Notes

- This example uses `provider.systemd.user-instance: true`, so Sablier talks to
  the **user** D-Bus. To manage system-level services instead, remove that line
  (or set it to `false`), install the units to `/etc/systemd/system`, and run the
  same `systemctl` commands **without** `--user` (these require `root`).
- Only units carrying `Enable=true` in an `[X-Sablier]` section can be managed.
- Sablier only discovers units that are **loaded** into the systemd manager. An
  enabled unit is always loaded, which is why `make install` enables
  `demo.service` even though it is not started until a session is requested.
