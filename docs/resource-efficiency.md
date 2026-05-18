# Resource Efficiency

Sablier saves resources by stopping containers when nobody is using them and
starting them back on demand.  This page explains how to measure those savings.

## How Sablier saves resources

Without Sablier, every container runs 24 hours a day whether it is being used
or not.  With Sablier, a container runs only when a request triggers it, then
stops automatically after a configurable idle timeout.

The saving is in the **idle dimension** — the hours per day a container is not
being used.  A typical homelab service might be accessed for 1–3 hours a day,
meaning it is idle 88–96 % of the time.  Stopping it during those idle hours
reclaims CPU, RAM, and the power that drives both.

## Enabling metrics

Add the following to your Sablier configuration:

```yaml
server:
  metrics:
    enabled: true
```

Metrics are then available at `http://<sablier-host>:10000/metrics`.

## The key metric: `sablier_instance_active_seconds_total`

Sablier exports a counter that accumulates the total seconds each instance has
spent in the **Ready** state:

```
sablier_instance_active_seconds_total{instance="<name>"}
```

The counter is incremented each time a session expires (i.e., the container is
stopped because it became idle).  It does **not** count startup time.

Use it to compute the **idle fraction** over any window — this is your
efficiency gain:

```promql
1 - (increase(sablier_instance_active_seconds_total{instance="myapp"}[24h]) / 86400)
```

Returns a value between `0` and `1`.  Example: `0.92` means the container was
stopped and consuming no resources 92 % of the day.

## Computing resource savings

Once you have the idle fraction, multiply by the container's idle cost:

$$\text{savings} = \text{idle fraction} \times \text{idle resource cost} \times N_{\text{containers}}$$

**Example — homelab with 5 containers**

| Measurement | Value |
|---|---|
| Average idle CPU per container | 4 % |
| Average idle RAM per container | 150 MB |
| Idle fraction | 92 % |
| Containers managed by Sablier | 5 |

CPU reclaimed per day:

$$0.92 \times 0.04 \times 5 \times 24\text{ h} = 4.4 \text{ CPU-hours/day}$$

RAM reclaimed when all containers are idle:

$$0.92 \times 150\text{ MB} \times 5 = 690\text{ MB}$$

To measure a container's idle cost, stop all traffic and run:

```bash
docker stats --no-stream --format \
  "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"
```

## What Sablier itself costs

Sablier's own process is lightweight — typically under 0.5 % CPU and 20 MB RAM
— and does not stop running between requests.  In practice its overhead is
negligible compared to the resources saved by even a single stopped container.

## See also

- [Metrics configuration](/configuration#metrics)
- [Performance benchmarks](/performance)
- [Tracing](/tracing)
