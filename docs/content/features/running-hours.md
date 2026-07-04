---
title: Running hours
weight: 2
compatibility:
  docker: supported
  swarm: supported
  kubernetes: differs
  podman: supported
  proxmox: unsupported
example: running-hours
---

{{< compatibility >}}

Use running-hours to keep an instance **available during specific daily hours**, proactively started and held warm for the whole window regardless of traffic.

Two labels drive it:

- **`sablier.running-hours`** — the daily window, 24-hour `HH:MM-HH:MM` (e.g. `09:00-18:00`). Overnight windows like `22:00-06:00` span midnight.
- **`sablier.running-days`** — optional, restricts the window to specific weekdays (comma-separated, full names or abbreviations, e.g. `Mon,Tue,Wed,Thu,Fri`). Defaults to every day.

Behavior:

- At the beginning of the window, Sablier proactively starts the instance.
- During the window, request-triggered sessions are extended to the window end so the instance is not stopped mid-window.
- After the window ends, normal session expiration resumes.
- For overnight windows, the day is evaluated against the day the window **starts** (a `Fri` + `22:00-06:00` window runs Friday 22:00 → Saturday 06:00).

## Setting the labels

{{< provider-tabs >}}
{{< provider-tab name="docker" >}}
```yaml
services:
  myapp:
    image: myapp:latest
    restart: unless-stopped
    labels:
      - "sablier.enable=true"
      - "sablier.group=myapp"
      - "sablier.running-hours=09:00-18:00"
      - "sablier.running-days=Mon,Tue,Wed,Thu,Fri"
```
{{< /provider-tab >}}
{{< provider-tab name="swarm" >}}
Same labels as Docker, placed under `deploy.labels`:

```yaml
services:
  myapp:
    image: myapp:latest
    deploy:
      labels:
        - "sablier.enable=true"
        - "sablier.group=myapp"
        - "sablier.running-hours=09:00-18:00"
        - "sablier.running-days=Mon,Tue,Wed,Thu,Fri"
```
{{< /provider-tab >}}
{{< provider-tab name="kubernetes" >}}
The `09:00-18:00` (colon) and `Mon,Tue,…` (comma) values are **not valid as Kubernetes label values**, so set them as **annotations**. Keep `sablier.enable` as a label so workload discovery (a label selector) still finds the workload.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    sablier.enable: "true"
  annotations:
    sablier.group: "myapp"
    sablier.running-hours: "09:00-18:00"
    sablier.running-days: "Mon,Tue,Wed,Thu,Fri"
```

{{< callout type="info" >}}
On Kubernetes, any Sablier value containing a colon or comma (`running-hours`, `running-days`, multi-value `group`) must be an **annotation**. Annotations take precedence over labels when both are present.
{{< /callout >}}
{{< /provider-tab >}}
{{< provider-tab name="podman" >}}
Identical to Docker — same labels.
{{< /provider-tab >}}
{{< /provider-tabs >}}

## Timezone (`TZ`)

Running-hours are evaluated in the process local timezone.

- In the official Docker image, the binary embeds timezone database data and supports `TZ` out of the box.
- The container defaults to `TZ=UTC`.
- Override with an environment variable, e.g. `-e TZ=Europe/Paris`.

## Format rules

- `sablier.running-hours`: 24-hour `HH:MM-HH:MM`. If start is later than end, the window spans midnight. Unparseable values are ignored.
- `sablier.running-days`: comma-separated days; full names (`Monday`) and abbreviations (`Mon`); case-insensitive; whitespace ignored. Unparseable values are ignored (the window then applies every day).
