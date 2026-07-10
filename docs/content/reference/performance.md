---
title: Performance
weight: 422
---

Sablier is designed to be a thin, low-overhead layer between your reverse proxy and your container runtime.  This page documents the measured overhead so you can reason about the impact on your workloads.

## Summary

| Scenario | Req/s (avg) | p50 | p95 | p99 |
|---|---|---|---|---|
| Blocking, warm session | 5,751 | 1.54 ms | 2.97 ms | 4.94 ms |
| Dynamic, warm session | 5,066 | 1.81 ms | 3.19 ms | 4.62 ms |
| Dynamic, not-ready (cold) | 4,663 | 1.93 ms | 3.57 ms | 5.88 ms |

At steady state (session cache hot, container already running), Sablier adds roughly **1.5-2 ms of latency** per request and sustains **~5,000-5,750 req/s** on a single modest core.

## Test environment

| Property | Value |
|---|---|
| CPU | Intel Core i7-8559U, 4 cores @ 2.70 GHz |
| OS | macOS, Docker Desktop |
| Sablier image | `sablierapp/sablier:latest` |
| Target image | `sablierapp/mimic:v0.3.3` (1 s simulated startup) |
| Load tool | [bombardier](https://github.com/codesenberg/bombardier) |

## Scenarios explained

### Blocking, warm session

The blocking strategy waits until the target container is ready before
responding. In the warm case the container is already running and the session is cached; every request goes through Sablier's routing, session lookup, and response generation only.

```
Reqs/sec      5750.89    (avg)    9003.48  (max)
Latency        1.74ms    (avg)   129.74ms  (max)
  p50   1.54ms  |  p75   1.93ms  |  p90   2.46ms
  p95   2.97ms  |  p99   4.94ms
2xx: 172,327   Throughput: 11.50 MB/s
```

### Blocking, cold start

The target container is stopped before each request, forcing a full cold start.
Bombardier runs with a single connection (`-c 1`) so measurements are
sequential and reflect the true end-to-end wait time a user would experience.
The first request triggers the container wakeup (about 1 s with mimic); subsequent
requests return immediately from the warm cache.

```
Latency    1.67s  (avg)    5.02s  (max)
  p50  822 µs  |  p90  5.02s  |  p99  5.02s
2xx: 3   (3 sequential requests)
```

> The wide spread (p50 = 822 microseconds, p90 = 5.02 s) is expected: with only 3
> requests the first bears the full container startup, while the remaining two
> hit the warm cache.  Cold start latency is dominated by the target's own
> startup time, not Sablier.

### Dynamic, warm session

The dynamic strategy always returns immediately with the container's current
readiness state.  The warm case shows the overhead of looking up the session
and rendering the ready-state response (HTML or JSON depending on the
`Accept` header).

```
Reqs/sec      5066.30    (avg)    7644.52  (max)
Latency        1.97ms    (avg)    94.89ms  (max)
  p50   1.81ms  |  p75   2.26ms  |  p90   2.77ms
  p95   3.19ms  |  p99   4.62ms
2xx: 151,980   Throughput: 22.23 MB/s
```

The higher throughput (22 MB/s vs 11 MB/s for blocking) reflects the richer
HTML body returned by the dynamic endpoint when the container is ready.

### Dynamic, not-ready (cold)

The target container is stopped before the run, so every request returns a
not-ready response immediately.  Comparing this to the warm dynamic run shows
the cost difference between the two rendering paths.

```
Reqs/sec      4662.66    (avg)    6969.98  (max)
Latency        2.15ms    (avg)   138.45ms  (max)
  p50   1.93ms  |  p75   2.42ms  |  p90   3.01ms
  p95   3.57ms  |  p99   5.88ms
2xx: 139,415   Throughput: 20.39 MB/s
```

The not-ready path is about **8% slower** in throughput and **~0.1 ms higher**
in p50 latency compared to the ready path, a negligible difference in practice.

## Key takeaways

- **Sablier's own overhead is ~1.5-2 ms** per request at steady state.  This
  is the cost of a session lookup, optional lock acquisition, and HTTP response
  generation.
- **Cold start latency is driven by the container**, not Sablier.  Once the
  target is ready, Sablier's per-request cost drops back to the warm baseline
  immediately.
- **Blocking vs dynamic** at steady state: blocking is ~13% faster in req/s
  (5,751 vs 5,066) because its response body is smaller.  Choose between them
  based on UX requirements, not performance.
- **Not-ready dynamic responses** cost essentially the same as ready responses
  (~8% throughput difference), so there is no performance reason to avoid
  hitting the dynamic endpoint while a container is starting.

## Reproducing the results

```bash
cd benchmarks/
make bench                         # run all four scenarios
make bench-blocking-warm           # individual scenario
SABLIER_IMAGE=sablierapp/sablier:{{< version >}} make bench
```

See the [benchmarks README](https://github.com/sablierapp/sablier/tree/main/benchmarks)
for full setup instructions.
