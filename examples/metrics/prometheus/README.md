# Prometheus metrics example

A minimal docker-compose stack showing Sablier exposing Prometheus metrics on
`/metrics` and being scraped by a Prometheus instance.

No reverse proxy — you drive Sablier directly with `curl` to its REST API and
watch the metrics appear in Prometheus.

## What it runs

| Service      | Image                       | Purpose                                            |
|--------------|-----------------------------|----------------------------------------------------|
| `sablier`    | `sablierapp/sablier`        | Sablier with `server.metrics.enabled: true`        |
| `whoami`     | `acouvreur/whoami:v1.10.2`  | Target service in group `demo` (sablier-managed)   |
| `prometheus` | `prom/prometheus:v3.1.0`    | Scrapes `sablier:10000/metrics` every 5s           |

The `whoami` container is labelled with `sablier.enable=true` and
`sablier.group=demo`, so Sablier will start and stop it on demand.

## Prerequisites

- Docker and `docker compose` v2

## Running

```sh
make up                # start the stack
make request-blocking  # ask Sablier to wake the "demo" group, wait until ready
make metrics           # print only the sablier_* metrics
```

Open the Prometheus UI at http://localhost:9090 to query the scraped metrics.

## Suggested demo loop

1. `make ps` — `whoami` starts running because docker compose creates it.
2. `make stop-target` — manually stop `whoami` so you can observe a cold
   start.
3. `make request-blocking` — Sablier warms `whoami` up, blocking until it
   reports ready. This populates
   `sablier_instance_start_duration_seconds`,
   `sablier_instance_ready_duration_seconds`, and
   `sablier_session_requests_total{strategy="blocking",target="group"}`.
4. Wait one minute (the `default-duration` configured in `sablier.yaml`).
   The session expires, Sablier stops `whoami`, and the metrics reflect this:
   `sablier_group_locked{group="demo"}` returns to `0`,
   `sablier_instance_stops_total{reason="expired"}` increments.

## Useful PromQL queries

| Query | What it shows |
|-------|---------------|
| `sablier_group_locked` | `1` while a group has at least one active session |
| `sablier_group_active_instances` | Active instance count per group |
| `histogram_quantile(0.95, rate(sablier_instance_ready_duration_seconds_bucket[5m]))` | p95 cold-start time across instances |
| `sablier_session_requests_total` | Cumulative API requests by strategy and target |
| `rate(sablier_instance_stops_total[5m])` | Stops per second by reason |

## Tearing down

```sh
make down
```

## Notes

- The `/metrics` endpoint requires a Sablier release that includes
  [PR #884](https://github.com/sablierapp/sablier/pull/884). The `image:`
  tag in `docker-compose.yml` carries an `# x-release-please-version` marker
  and will be auto-bumped on the next release. To run against an unreleased
  build, replace the `image:` directive with a `build:` directive pointing
  at the parent repo, or use the `:dev` tag if your CI publishes one.
- The example deliberately omits a reverse proxy. Adding Traefik, Caddy,
  Nginx, or Envoy is covered in the main Sablier docs.
