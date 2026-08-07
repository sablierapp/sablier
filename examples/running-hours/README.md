# `sablier.running-hours` Example

This example demonstrates how `sablier.running-hours` keeps an instance warm
within a daily time window.

When the current time is inside the window:

- Sablier proactively starts the instance during reconciliation.
- Any request-triggered session is extended to the window end.

When the current time is outside the window:

- The instance behaves normally and can expire according to session settings.

The optional `sablier.running-days` label further restricts the window to
specific weekdays (this example uses `Mon,Tue,Wed,Thu,Fri`).

## Services

| Service | Role |
|---|---|
| `sablier` | Manages Docker containers and exposes API on `:10000` |
| `office-app` | Managed app with `sablier.running-hours` |

## Labels on `office-app`

```yaml
labels:
  - "sablier.enable=true"
  - "sablier.group=office-app"
  - "sablier.running-hours=09:00-18:00"
  - "sablier.running-days=Mon,Tue,Wed,Thu,Fri"
```

## Prerequisites

- Docker with Compose plugin (`docker compose version`)
- `curl` and `jq` for API walkthrough

## Configure timezone and running-hours

Running-hours are evaluated in Sablier process local time.

This compose stack supports both values as environment variables:

```bash
export TZ=Europe/Paris
export RUNNING_HOURS=09:00-18:00
export RUNNING_DAYS=Mon,Tue,Wed,Thu,Fri
```

You can keep defaults by leaving them unset (`TZ=UTC`, `RUNNING_HOURS=09:00-18:00`, `RUNNING_DAYS=Mon,Tue,Wed,Thu,Fri`).

`RUNNING_DAYS` accepts a comma-separated list of days (full names like `Monday` or abbreviations like `Mon`).

## Walkthrough

### 1. Start the stack

```bash
make up
```

### 2. Watch logs in another terminal

```bash
make logs
```

### 3. Trigger a blocking request

```bash
make request
```

If the current time is inside `RUNNING_HOURS`, logs should include a message like:

```text
running-hours window active, extending expiration
```

### 4. Check container status

```bash
make status
```

`office-app` should remain up while inside the running-hours window.

### 5. Tear down

```bash
make down
```
