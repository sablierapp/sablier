## Sablier Healthcheck

### Using the `/health` route

You can use the route `/health` to check for healthiness.

- Returns 200 `OK` when ready
- Returns 503 `Service Unavailable` when terminating

### Using the `sablier health` command

You can use the command `sablier health` to check for healthiness.

`sablier health` takes on argument `--url` which defaults to `http://localhost:10000/health`.

```yml
services:
  sablier:
    image: sablierapp/sablier:1.10.0
    healthcheck:
      test: ["sablier", "health"]
      interval: 1m30s
```