# Nomad Provider Example

This example runs Sablier with the **Nomad provider**. Sablier scales the task group `web` of the Nomad job `whoami` from 0 to 1 when a session starts, and back to 0 when the session expires.

Docker Compose runs a Nomad agent in development mode and Sablier. The task of the job is the busybox web server that is part of the Nomad image. You do not need to install Nomad, and the example does not pull other images.

```
+--------+  blocking request   +-----------+   scaling API    +---------------------------+
|  curl  | ------------------> |  sablier  | ---------------> |  nomad (development mode) |
+--------+  names=whoami/web   |  :10000   | <--------------- |  job "whoami"             |
    |                          +-----------+   event stream   |    group "web"  (0 to 1)  |
    |                                                         |      task "server" :8080  |
    +-------------------- GET http://localhost:8080 --------> +---------------------------+
```

## Services

| Service | Role |
|---|---|
| `nomad` | Nomad agent in development mode, which is a server and a client in one process. The API and the UI are on port `4646`. The task group `whoami/web` listens on port `8080`. |
| `sablier` | Sablier with the Nomad provider. The API is on port `10000`. |

## Files

| File | Purpose |
|---|---|
| `compose.yml` | Runs the Nomad agent and Sablier |
| `raw_exec.hcl` | Nomad agent configuration that enables the `raw_exec` driver |
| `whoami.nomad.hcl` | The job that Sablier manages |
| `Makefile` | The `up`, `down`, `start`, `status` and `logs` targets |

## The job

```hcl
group "web" {
  count = 0

  meta = {
    "sablier.enable" = "true"
    "sablier.group"  = "demo"
  }

  update {
    health_check     = "checks"
    min_healthy_time = "5s"
  }

  service {
    provider = "nomad"
    check {
      type = "http"
      path = "/"
    }
  }
}
```

- `count = 0` keeps the task group stopped until a session starts.
- Use the map syntax `meta = { ... }`. The label names contain dots, and HCL does not accept quoted argument names in a `meta { ... }` block.
- The service has an HTTP check, and the `update` block sets `min_healthy_time`. Sablier reports the instance as ready only after the deployment marks the allocation healthy.

## Prerequisites

- Docker with the Compose plugin. The Nomad container runs privileged and uses the cgroup namespace of the host, so use Docker on Linux or Docker Desktop.
- `make`, `curl` and `jq`.
- A Sablier image with the Nomad provider. The Compose file uses `sablierapp/sablier:latest`. If that image does not have the Nomad provider yet, build a local image from the root of the repository and set `SABLIER_IMAGE`:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags nomsgpack -o dist/linux/amd64/sablier ./cmd/sablier
docker build --platform linux/amd64 -f build/Dockerfile -t sablierapp/sablier:local dist
export SABLIER_IMAGE=sablierapp/sablier:local
```

## Walkthrough

### 1. Start the stack

```bash
make up
```

Docker Compose starts Nomad and Sablier. Then the Nomad CLI in the `nomad` container registers the job with `count = 0`. Open http://localhost:4646/ui/jobs/whoami@default to see the job in the Nomad UI.

### 2. Request a session (blocking strategy)

```bash
make start
```

Sablier scales `whoami/web` to 1 and waits until the deployment marks the allocation healthy. The request returns after about 10 seconds:

```json
{
  "session": {
    "instances": [
      {
        "instance": {
          "name": "whoami/web",
          "currentReplicas": 1,
          "desiredReplicas": 1,
          "status": "ready",
          "groups": ["demo"],
          "enabled": "true",
          "provider": "nomad",
          "nomad": {
            "namespace": "default",
            "jobId": "whoami",
            "taskGroup": "web",
            "meta": {
              "sablier.enable": "true",
              "sablier.group": "demo"
            }
          },
          "readyAt": "2026-09-11T03:58:54.495711918Z",
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

### 3. Call the task group

```bash
curl http://localhost:8080
```

```
Hello from the Nomad task group whoami/web.
Allocation dbbb739f-9d3d-59ef-eb20-d7cbd468be8e was started on demand by Sablier.
```

### 4. Check the job

```bash
make status
```

The deployment is successful, and one allocation runs:

```
Deployed
Task Group  Desired  Placed  Healthy  Unhealthy  Progress Deadline
web         1        1       1        0          2026-09-11T04:08:51Z

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created  Modified
dbbb739f  44d4e0b1  web         1        run      running  11s ago  4s ago
```

### 5. Watch the scale down

The session lasts 1 minute, and Sablier looks for expired sessions every 10 seconds. Wait, then check the job again:

```bash
sleep 80
make status
```

Sablier scales the task group back to 0, so the allocation stops. After that, `curl http://localhost:8080` gets no response.

```
Allocations
ID        Node ID   Task Group  Version  Desired  Status    Created    Modified
dbbb739f  44d4e0b1  web         1        stop     complete  1m31s ago  20s ago
```

### 6. Tear down

```bash
make down
```

## What to look for in the logs

```bash
make logs
```

Sablier gets the job from the Nomad event stream and adds it to the group `demo`. Then it scales the task group up, waits until it is ready, and scales it down when the session expires:

```
INF connection established with Nomad provider=nomad address=http://nomad:4646 namespace=default
DBG instance event provider=nomad type=created name=whoami/web
INF instance added to group instance=whoami/web group=demo reason=created
DBG scaling task group up provider=nomad name=whoami/web replicas=1
DBG task group inspected provider=nomad name=whoami/web status=starting current_replicas=0 desired_replicas=1
DBG task group inspected provider=nomad name=whoami/web status=ready current_replicas=1 desired_replicas=1
INF instance expired instance=whoami/web
DBG scaling task group to zero provider=nomad name=whoami/web
DBG instance event provider=nomad type=stopped name=whoami/web
```

## Notes

- Development mode and the `raw_exec` driver are for local tests only. In production, run a Nomad cluster with ACLs, use a driver that isolates tasks, and give Sablier an ACL token. See the [Nomad provider tutorial](https://sablierapp.dev/tutorials/providers/nomad/).
- Add health checks to the services of your task groups. Without a deployment that tracks health, Sablier reports a running allocation as ready at once.
- A reverse proxy plugin uses the same instance name, `whoami/web`, or the group `demo`.
