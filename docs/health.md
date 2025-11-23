## Sablier Healthcheck

### Using the `/health` Route

You can use the `/health` route to check for service health.

- Returns 200 `OK` when ready
- Returns 503 `Service Unavailable` when terminating

### Using the `sablier health` Command

You can use the `sablier health` command to check for service health.

The `sablier health` command takes one argument, `--url`, which defaults to `http://localhost:10000/health`.

<!-- x-release-please-start-version -->
```yml
services:
  sablier:
    image: sablierapp/sablier:1.10.5
    healthcheck:
      test: ["sablier", "health"]
      interval: 1m30s
```
<!-- x-release-please-end -->