# Sablier Benchmarks

End-to-end HTTP benchmarks for the Sablier API using
[bombardier](https://github.com/codesenberg/bombardier) as the load generator.

## Prerequisites

- Docker (with access to `/var/run/docker.sock`)
- Go 1.24+ (for `go tool` support in `tools.mod`)

## Quick start

```bash
cd benchmarks/

make bench          # run all four scenarios in sequence
# or individually:
make bench-blocking-warm
make bench-blocking-cold
make bench-dynamic-warm
make bench-dynamic-cold
```

`make up` is called automatically, so you do not need to start the stack manually.

## Scenarios

There are four scenarios, covering both strategies × both container states.
Together they show the overhead of each endpoint independently of each other.

| Target | `make` target | Connection model |
|---|---|---|
| blocking, container running | `bench-blocking-warm` | 10 conns × 30 s |
| blocking, cold start | `bench-blocking-cold` | 1 conn × N requests |
| dynamic, container running | `bench-dynamic-warm` | 10 conns × 30 s |
| dynamic, container stopped | `bench-dynamic-cold` | 10 conns × 30 s |

### Blocking warm (`make bench-blocking-warm`)

Measures Sablier's **per-request overhead for the blocking strategy** when the
target is already running and the session cache is hot.  The session is primed
with a single curl before bombardier starts, so every measured request hits
only Sablier's routing and session-lookup code path.

### Blocking cold (`make bench-blocking-cold`)

Measures the **full round-trip** including Docker container start time.  The
target is stopped before each request so every hit triggers a cold start
through the blocking strategy (waits up to 120 s for readiness).

```bash
make bench-blocking-cold          # 3 samples (default)
make bench-blocking-cold N=10     # override sample count
```

Because each request waits for the container, bombardier runs with a single
connection (`-c 1`) so requests are sequential rather than piled up.

### Dynamic warm (`make bench-dynamic-warm`)

Measures Sablier's **response-rendering overhead for the dynamic strategy**
when the target is already running.  The dynamic endpoint always returns
immediately (it never blocks), so this isolates the cost of looking up the
session state and rendering the ready-state response.

### Dynamic cold (`make bench-dynamic-cold`)

Measures the **not-ready rendering path** of the dynamic strategy.  The target
is stopped before the run, so every request returns immediately with a
not-ready response.  Comparing this to `bench-dynamic-warm` shows the
difference in cost between the two rendering paths.

## Latest results

Measured on an Intel Core i7-8559U (4 cores, 2.70 GHz), macOS, Docker Desktop,
`sablierapp/sablier:latest`, `sablierapp/mimic:v0.3.3` (1 s simulated startup).

### Summary

| Scenario | Req/s (avg) | Latency p50 | Latency p95 | Latency p99 | Max latency |
|---|---|---|---|---|---|
| Blocking — warm | 5,751 | 1.54 ms | 2.97 ms | 4.94 ms | 129.74 ms |
| Blocking — cold | — *(3 samples)* | 822 µs | 5.02 s | 5.02 s | 5.02 s |
| Dynamic — warm | 5,066 | 1.81 ms | 3.19 ms | 4.62 ms | 94.89 ms |
| Dynamic — cold | 4,663 | 1.93 ms | 3.57 ms | 5.88 ms | 138.45 ms |

> **Cold blocking** uses sequential requests (`-c 1`) so the numbers reflect
> real end-to-end container wake-up time, not concurrency effects.  The first
> request bore the full 1 s mimic startup; subsequent requests hit the warm cache.

### Blocking — warm

```
Statistics        Avg      Stdev        Max
  Reqs/sec      5750.89    1552.28    9003.48
  Latency        1.74ms     1.32ms   129.74ms
  Latency Distribution
     50%     1.54ms
     75%     1.93ms
     90%     2.46ms
     95%     2.97ms
     99%     4.94ms
  HTTP codes:
    1xx - 0, 2xx - 172327, 3xx - 0, 4xx - 0, 5xx - 0
  Throughput:    11.50MB/s
```

### Blocking — cold (3 sequential requests)

```
Statistics        Avg      Stdev        Max
  Reqs/sec        35.20     551.01    8764.55
  Latency         1.67s      2.37s      5.02s
  Latency Distribution
     50%   822.00us
     75%   822.00us
     90%      5.02s
     95%      5.02s
     99%      5.02s
  HTTP codes:
    1xx - 0, 2xx - 3, 3xx - 0, 4xx - 0, 5xx - 0
```

### Dynamic — warm

```
Statistics        Avg      Stdev        Max
  Reqs/sec      5066.30     963.27    7644.52
  Latency        1.97ms     0.90ms    94.89ms
  Latency Distribution
     50%     1.81ms
     75%     2.26ms
     90%     2.77ms
     95%     3.19ms
     99%     4.62ms
  HTTP codes:
    1xx - 0, 2xx - 151980, 3xx - 0, 4xx - 0, 5xx - 0
  Throughput:    22.23MB/s
```

### Dynamic — cold

```
Statistics        Avg      Stdev        Max
  Reqs/sec      4662.66    1087.19    6969.98
  Latency        2.15ms     1.61ms   138.45ms
  Latency Distribution
     50%     1.93ms
     75%     2.42ms
     90%     3.01ms
     95%     3.57ms
     99%     5.88ms
  HTTP codes:
    1xx - 0, 2xx - 139415, 3xx - 0, 4xx - 0, 5xx - 0
  Throughput:    20.39MB/s
```

## Using a specific Sablier image

```bash
SABLIER_IMAGE=sablierapp/sablier:v1.8.0 make bench-warm
```

The default is `sablierapp/sablier:latest`.

## Interpreting the output

bombardier prints a summary like:

```
Statistics        Avg      Stdev        Max
  Reqs/sec      1842.35    213.40    2401.77
  Latency        5.43ms     1.12ms    32.18ms
  HTTP codes:
    1xx - 0, 2xx - 55271, 3xx - 0, 4xx - 0, 5xx - 0
  Throughput:   512.00KB/s
```

Key fields:

| Field | What it tells you |
|---|---|
| **Reqs/sec (Avg)** | Sablier's sustainable throughput for the scenario |
| **Latency (Avg)** | Median per-request cost added by Sablier |
| **Latency (Max)** | Worst-case spike — watch for outliers in cold runs |
| **2xx count** | All requests should be 2xx; any 4xx/5xx signals a config problem |
| **Throughput** | Useful only for warm runs; cold runs are dominated by wait time |

**Blocking warm / Dynamic warm**: focus on `Reqs/sec` and `Latency (Avg)`.
These reflect Sablier's steady-state overhead.  Comparing the two reveals the
extra cost the blocking strategy adds over dynamic for hot sessions (session
lock, timeout accounting).

**Blocking cold**: focus on `Latency (Avg)` and `Latency (Max)`.  These
capture the real end-to-end wait a user experiences when a sleeping container
wakes up, including the 1 s simulated startup delay from `sablierapp/mimic`.

**Dynamic cold vs dynamic warm**: the latency delta between the two shows how
much more (or less) expensive the not-ready rendering path is compared to the
ready path.

## Teardown

```bash
make down    # stop and remove containers
make clean   # down + remove local images pulled during the run
```
