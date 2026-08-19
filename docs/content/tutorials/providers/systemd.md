---
title: Systemd
weight: 6
---

This tutorial connects Sablier to systemd. You will select the Systemd provider, give Sablier access to the system or user systemd bus, register a unit so Sablier can manage it, and confirm Sablier knows when the unit is ready.

## Select the Systemd provider

Set the [provider.name](/reference/cli/) property to `systemd`.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: systemd
  systemd:
    user-instance: false
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.name=systemd
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=systemd
```

{{< /tab >}}
{{< /tabs >}}

If you manage per-user services, enable the user bus instead:

```yaml
provider:
  systemd:
    user-instance: true
```

The equivalent environment variable is `SABLIER_PROVIDER_SYSTEMD_USER_INSTANCE=true`.

Run Sablier in the same host and systemd scope as the units it manages. A user instance requires access to that user's D-Bus and unit files; a system instance requires permission to call the system manager.

## Register a unit

For Sablier to work, it needs to know which systemd unit to start and stop. Register a unit by adding an `X-Sablier` section:

```ini
[Unit]
Description=whoami

[Service]
ExecStart=/usr/local/bin/whoami

[X-Sablier]
Enable=true
Group=mygroup
```

`Enable=true` is required. The provider refuses to start or stop units that have not explicitly opted in, preventing requests from controlling unrelated host services.

Reload systemd after changing the file:

```bash
systemctl daemon-reload
```

Use `systemctl --user daemon-reload` for a user instance.

Sablier accepts the same settings documented in the [labels reference](/reference/labels/) without the `sablier.` prefix and in systemd-style casing. For example, `sablier.ready-after` becomes `ReadyAfter`, and `sablier.idle.cpu` becomes `IdleCPU`.

## Runnable example

A complete, runnable example (using the systemd **user** instance, no `sudo`
required) is available in
[`examples/systemd`](https://github.com/sablierapp/sablier/tree/main/examples/systemd).

## Confirm when the unit is ready

Sablier waits for systemd to report the unit as active before marking it `ready`.

{{< callout type="info" >}}
Systemd units have no labels; Sablier reads the `[X-Sablier]` section from the unit file instead.
{{< /callout >}}
