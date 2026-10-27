---
title: Systemd
weight: 6
since: v1.18.0
aliases:
  - /tutorials/providers/systemd/
---

## Introduction

The systemd provider starts [units](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html) when a request needs them, and stops them when their session expires. Sablier runs on the same host and talks to the [service manager](https://www.freedesktop.org/software/systemd/man/latest/systemd.html) over its [D-Bus API](https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html). It reads its settings from an `[X-Sablier]` section in each unit file.

## Install Sablier

Run Sablier on the host, as a [service](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html) of the manager whose units it controls. Sablier reads each unit file at the path systemd reports, so it needs the host's file system.

{{< tabs >}}
{{< tab name="System instance" >}}

Download the binary from the [releases page](https://github.com/sablierapp/sablier/releases). The command fetches the `amd64` build. On ARM, use `arm64` or `armv7` instead:

```bash
curl -fsSL https://github.com/sablierapp/sablier/releases/download/v{{< version >}}/sablier-{{< version >}}-linux-amd64.tar.gz \
  | sudo tar -xz -C /usr/local/bin sablier
```

Select the systemd provider in `/etc/sablier/sablier.yaml`:

{{< example file="systemd/system/sablier.yaml" >}}

Run Sablier as a system [service](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html):

{{< example file="systemd/system/sablier.service" >}}

```bash
sudo install -D -m 0644 sablier.yaml /etc/sablier/sablier.yaml
sudo install -m 0644 sablier.service /etc/systemd/system/sablier.service
sudo systemctl daemon-reload
sudo systemctl enable --now sablier.service
```

Sablier connects to the system [D-Bus](https://www.freedesktop.org/wiki/Software/dbus/). Starting, stopping and throttling units needs root, or a [polkit](https://www.freedesktop.org/software/polkit/docs/latest/polkit.8.html) rule that grants [`org.freedesktop.systemd1.manage-units`](https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html#Security).

{{< /tab >}}
{{< tab name="User instance" >}}

Download the binary from the [releases page](https://github.com/sablierapp/sablier/releases) into your home directory. The command fetches the `amd64` build. On ARM, use `arm64` or `armv7` instead:

```bash
mkdir -p ~/.local/bin
curl -fsSL https://github.com/sablierapp/sablier/releases/download/v{{< version >}}/sablier-{{< version >}}-linux-amd64.tar.gz \
  | tar -xz -C ~/.local/bin sablier
```

Select the systemd provider and the user instance in `~/.config/sablier/sablier.yaml`:

{{< example file="systemd/sablier.yaml" >}}

Run Sablier as a unit of your [user manager](https://www.freedesktop.org/software/systemd/man/latest/user@.service.html):

{{< example file="systemd/sablier.service" >}}

```bash
install -D -m 0644 sablier.yaml ~/.config/sablier/sablier.yaml
install -D -m 0644 sablier.service ~/.config/systemd/user/sablier.service
systemctl --user daemon-reload
systemctl --user enable --now sablier.service
loginctl enable-linger
```

Sablier connects to your session [D-Bus](https://www.freedesktop.org/wiki/Software/dbus/) and manages the units of your user manager, without root. [Lingering](https://www.freedesktop.org/software/systemd/man/latest/loginctl.html#enable-linger%20USER%E2%80%A6) keeps the user manager, and Sablier, running after you log out.

{{< /tab >}}
{{< /tabs >}}

Reverse proxy plugins on the same host reach Sablier at `http://localhost:10000`.

## Usage example

Opt a [unit](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html) in with an `[X-Sablier]` section, and give it a group:

{{< example file="systemd/demo.service" >}}

Install it, [reload](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#daemon-reload) systemd, and [enable](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#enable%20UNIT%E2%80%A6) the unit without starting it:

```bash
sudo install -m 0644 demo.service /etc/systemd/system/demo.service
sudo systemctl daemon-reload
sudo systemctl enable demo.service
```

An enabled unit stays [loaded](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#Unit%20Garbage%20Collection), so Sablier finds it in its group. With the user instance, install it in `~/.config/systemd/user/` and use [`systemctl --user`](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#--user) without `sudo`.

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `demo` group in front of your service. On the first request, Sablier starts `demo.service` and holds the request, or shows a waiting page, until systemd reports the unit [active](#readiness). When the session expires, Sablier stops it.

To try it without a reverse proxy, request the group from the Sablier API:

```bash
curl "http://localhost:10000/api/strategies/blocking?group=demo&session_duration=1m"
```

{{< callout type="info" >}}
Sablier stops opted-in units that are active when it starts. A unit you start afterwards keeps running until its first session expires, unless you [stop externally-started instances](/features/stop-external-starts/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.systemd.user-instance`](/reference/cli/#opt-provider-systemd-user-instance) | `false` | Manage the units of the [user manager](https://www.freedesktop.org/software/systemd/man/latest/user@.service.html) of the user Sablier runs as, instead of the system manager. |
| [`--provider.systemd.unit-patterns`](/reference/cli/#opt-provider-systemd-unit-patterns) | `[]` | Only consider units whose name matches one of these [glob patterns](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#Parameter%20Syntax), such as `app-*.service`. |

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/).

`unit-patterns` narrows which units Sablier lists and watches:

- **A matching unit still needs `Enable=true`.** Patterns do not opt units in.
- **Patterns make listing cheaper.** systemd filters the units, so Sablier reads fewer unit files.
- **Patterns do not filter requests by [instance name](#instance-names).** A request for a unit outside the patterns still starts it if it has `Enable=true`.

### Workloads

#### Units

Sablier starts and stops [units](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html) through the manager's [D-Bus API](https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html), as `systemctl start` and `systemctl stop` do. Most of the time, the unit is a [service](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html).

- **Start** applies the active [resource limits](#scale-instead-of-stop), if any, then starts the unit. Sablier waits until systemd finishes the start, and reports an error if it fails.
- **Stop** stops the unit and waits until systemd finishes. With [`IdleReplicas=1`](#scale-instead-of-stop), Sablier applies the idle limits instead and leaves the unit running.

### Instance names

A reverse proxy can target one unit by name instead of a group. The unit of the usage example is named:

```text
demo.service
```

- **Use the full unit name, with its suffix.** Sablier sends the name to systemd as is: unlike [`systemctl`](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#Parameter%20Syntax), it does not add `.service`.
- **The name refers to the manager Sablier talks to.** With `user-instance`, it is a unit of the user manager.

### Labels

Systemd [units](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html) have no labels. Sablier reads its settings from an `[X-Sablier]` section in the unit file:

{{< example file="systemd/demo.service" region="sablier" >}}

- **Use the exact key names below.** Keys are case-sensitive, and Sablier ignores any other key.
- **systemd ignores the section.** It [skips sections whose name starts with `X-`](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#Description), so the unit loads without warnings.
- **Set each key once.** If a key repeats, the last value wins. List several values on one line, separated by commas.
- **Put the section in the unit file itself.** Sablier reads the file systemd reports as the unit's [`FragmentPath`](https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html#Properties1), never its [drop-ins](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#Description).
- **Sablier picks up `[X-Sablier]` changes on its own.** It reads the unit file again when the file changes. Run [`systemctl daemon-reload`](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#daemon-reload) for changes to the other sections.

The key names do not follow the labels' casing, so use this table:

| Label | `[X-Sablier]` key |
|---|---|
| [`sablier.enable`](/reference/labels/#label-sablier-enable) | `Enable` |
| [`sablier.group`](/reference/labels/#label-sablier-group) | `Group` |
| [`sablier.ready-after`](/reference/labels/#label-sablier-ready-after) | `ReadyAfter` |
| [`sablier.ready-on-start`](/reference/labels/#label-sablier-ready-on-start) | `ReadyOnStart` |
| [`sablier.running-hours`](/reference/labels/#label-sablier-running-hours) | `RunningHours` |
| [`sablier.running-days`](/reference/labels/#label-sablier-running-days) | `RunningDays` |
| [`sablier.anti-affinity`](/reference/labels/#label-sablier-anti-affinity) | `AntiAffinity` |
| `sablier.idle.replicas`, `sablier.active.replicas` | `IdleReplicas`, `ActiveReplicas` |
| `sablier.idle.cpu`, `sablier.active.cpu` | `IdleCPU`, `ActiveCPU` |
| `sablier.idle.memory`, `sablier.active.memory` | `IdleMemory`, `ActiveMemory` |
| `sablier.idle.blkio-weight`, `sablier.active.blkio-weight` | `IdleBlkioWeight`, `ActiveBlkioWeight` |
| `sablier.idle.blkio-weight-device`, `sablier.active.blkio-weight-device` | `IdleBlkioWeightDevice`, `ActiveBlkioWeightDevice` |
| `sablier.idle.blkio-device-read-bps`, `sablier.active.blkio-device-read-bps` | `IdleBlkioDeviceReadBps`, `ActiveBlkioDeviceReadBps` |
| `sablier.idle.blkio-device-write-bps`, `sablier.active.blkio-device-write-bps` | `IdleBlkioDeviceWriteBps`, `ActiveBlkioDeviceWriteBps` |
| `sablier.idle.blkio-device-read-iops`, `sablier.active.blkio-device-read-iops` | `IdleBlkioDeviceReadIOps`, `ActiveBlkioDeviceReadIOps` |
| `sablier.idle.blkio-device-write-iops`, `sablier.active.blkio-device-write-iops` | `IdleBlkioDeviceWriteIOps`, `ActiveBlkioDeviceWriteIOps` |

The [label reference](/reference/labels/) describes every setting.

### Workload settings

Each excerpt below is the `[X-Sablier]` section of a complete unit file in [`examples/systemd`](https://github.com/sablierapp/sablier/tree/main/examples/systemd).

#### Enable {since="v1.18.0"}

`Enable=true` opts the unit in. It is required, and any other value is ignored. Sablier refuses to start or stop a unit without it, so a request cannot control unrelated host services. See the [usage example](#usage-example).

#### Group {since="v1.18.0"}

`Group` puts the unit in one or more groups, separated by commas. A session for any of them starts it. Without it, the unit is in the `default` group.

{{< example file="systemd/group.service" region="sablier" >}}

[Learn more about groups](/features/groups/).

#### Ready after {since="v1.18.0"}

`ReadyAfter` keeps reporting the unit as not ready for a settling delay after systemd first reports it [active](#readiness). The value is a Go duration, such as `30s`.

{{< example file="systemd/ready-after.service" region="sablier" >}}

Use it when your service is active before it can serve traffic. With [`Type=simple` or `Type=exec`](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html#Type=), a service is active as soon as its process starts.

[Learn more about delaying readiness](/features/ready-after/).

#### Ready on start {since="v1.18.0"}

`ReadyOnStart=true` reports the unit ready as soon as Sablier asks systemd to start it, without waiting for it to be [active](#readiness).

{{< example file="systemd/ready-on-start.service" region="sablier" >}}

[Learn more about marking ready on start](/features/ready-on-start/).

#### Running hours {since="v1.18.0"}

- `RunningHours` keeps the unit running during a daily window.
- `RunningDays` limits that window to some weekdays.

{{< example file="systemd/running-hours.service" region="sablier" >}}

The window uses Sablier's local time: the host's time zone, unless you set `TZ` in the [`Environment=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#Environment=) of Sablier's unit.

[Learn more about running hours](/features/running-hours/).

#### Anti-affinity {since="v1.18.0"}

`AntiAffinity` lists groups the unit backs off from, separated by commas. Sablier keeps it idle while any of them has an active session.

{{< example file="systemd/anti-affinity.service" region="sablier" >}}

[Learn more about anti-affinity](/features/anti-affinity/).

#### Scale instead of stop {since="v1.18.0"}

Keep the unit running when idle, with lower limits, instead of stopping it:

- `IdleReplicas=1` keeps the unit running when idle.
- `IdleCPU` and `IdleMemory` set its CPU and memory limits while idle.
- `ActiveCPU` and `ActiveMemory` set them for the next session.

{{< example file="systemd/scale-mode.service" region="sablier" >}}

Sablier sets the limits as [resource control](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html) properties of the unit:

- **CPU is in decimal cores and becomes [`CPUQuota=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#CPUQuota=).** `0.1` is 10%, `2` is 200%.
- **Memory uses Docker units and becomes [`MemoryMax=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#MemoryMax=bytes).** For example, `64m` or `1g`.
- **The running unit keeps running.** systemd applies the new limits immediately, without a restart.
- **Limits are runtime properties.** Sablier sets them as [`systemctl set-property --runtime`](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#--runtime) does: your unit file does not change, and a reboot drops them.
- **A limit set on one side only is reset on the other.** If you set `IdleMemory` alone, the next session sets `MemoryMax=infinity`, and the next expiry sets it back. If your unit file sets a limit, repeat it in the matching `Active` key.
- **Active limits apply on every start**, even when `IdleReplicas` is `0` and the unit is stopped when idle.
- **One unit runs at most.** `IdleReplicas` and `ActiveReplicas` above `1` count as `1`, and Sablier logs a warning.

[Learn more about scaling instead of stopping](/features/scale-mode/).

#### Block I/O {since="v1.18.0"}

With `IdleReplicas=1`, throttle the unit's block I/O while idle:

{{< example file="systemd/blkio.service" region="sablier" >}}

Each key pair sets one resource control property:

| Keys | systemd property | Value |
|---|---|---|
| `IdleBlkioWeight`, `ActiveBlkioWeight` | [`IOWeight=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#IOWeight=weight) | Integer from `10` to `1000` |
| `IdleBlkioWeightDevice`, `ActiveBlkioWeightDevice` | [`IODeviceWeight=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#IODeviceWeight=device%20weight) | `path:weight`, weight from `10` to `1000` |
| `IdleBlkioDeviceReadBps`, `ActiveBlkioDeviceReadBps` | [`IOReadBandwidthMax=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#IOReadBandwidthMax=device%20bytes) | `path:rate`, rate in Docker units |
| `IdleBlkioDeviceWriteBps`, `ActiveBlkioDeviceWriteBps` | [`IOWriteBandwidthMax=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#IOReadBandwidthMax=device%20bytes) | `path:rate`, rate in Docker units |
| `IdleBlkioDeviceReadIOps`, `ActiveBlkioDeviceReadIOps` | [`IOReadIOPSMax=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#IOReadIOPSMax=device%20IOPS) | `path:iops`, iops an integer |
| `IdleBlkioDeviceWriteIOps`, `ActiveBlkioDeviceWriteIOps` | [`IOWriteIOPSMax=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#IOReadIOPSMax=device%20IOPS) | `path:iops`, iops an integer |

- **Separate devices with commas.** For example, `/dev/sda:10m,/dev/sdb:5m`.
- **Sablier ignores a weight outside `10`-`1000`.** systemd accepts up to `10000`, and its default is `100`.
- **Rates use Docker units, in bytes per second.** `10m` is 10 MiB/s.
- **Omitted per-device limits are cleared on the switch.** Sablier replaces the whole device list, so the idle limits above are removed when a session starts.
- **These are runtime properties**, like the [CPU and memory limits](#scale-instead-of-stop).

[Learn more about throttling block I/O](/features/disk-io-limits/).

### Readiness

Sablier reports a unit from its [`ActiveState`](https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html#Properties1):

- **ready** when it is `active`.
- **starting** when it is `activating`, `reloading` or `refreshing`.
- **stopped** when it is `inactive`, `deactivating` or `maintenance`.
- **error** when it is `failed`.

When a service becomes active depends on its [`Type=`](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html#Type=). With `Type=notify`, it is active once it reports ready through [`sd_notify`](https://www.freedesktop.org/software/systemd/man/latest/sd_notify.html). With `Type=simple`, it is active as soon as its process starts: add a [ready-after](#ready-after) delay if it needs time to serve traffic.

### Limitations

- Sablier must run on the host of the units it manages: it reads their unit files from disk.
- One Sablier manages either the system manager or one [user manager](https://www.freedesktop.org/software/systemd/man/latest/user@.service.html), not both.
- Groups only contain units systemd has [loaded](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#Unit%20Garbage%20Collection). Enable a unit so it stays loaded.
- Sablier ignores `[X-Sablier]` sections in [drop-in files](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#Description).
- At most one instance of a unit runs: replica values above `1` count as `1`.
- Resource limits need the [unified cgroup hierarchy](https://docs.kernel.org/admin-guide/cgroup-v2.html). On a user instance, they only apply for the controllers [delegated](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#Delegate=) to the user manager.
- Sablier polls systemd every 10 seconds for units started or stopped outside Sablier.

## Examples

- [`examples/systemd`](https://github.com/sablierapp/sablier/tree/main/examples/systemd): every unit file on this page, with a `Makefile` to run them on your user instance.
