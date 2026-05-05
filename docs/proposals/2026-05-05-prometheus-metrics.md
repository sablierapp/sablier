# Proposal: Prometheus `/metrics` endpoint

- Date: 2026-05-05
- Status: Draft

## Summary

Expose a Prometheus-compatible `/metrics` endpoint on the existing Sablier
HTTP server. Track group lock state (groups with at least one active session),
per-instance warmup time (both provider start duration and end-to-end
not-ready→ready wall time), session request counts, instance start failures,
instance stops, and the standard Go runtime + process collectors.

The endpoint is opt-in through configuration and disabled by default.

## Motivation

Operators running Sablier in production currently have no built-in visibility
into how the system is behaving:

- Which groups are actively being held warm by user traffic right now?
- How long does it take for a group to warm up after the first request hits
  it? (This is the user-perceived latency on the blocking strategy.)
- Are provider start calls failing? How often?
- Is the Sablier process itself healthy (heap, goroutines, GC, CPU)?

These questions are easy to answer when Sablier exposes Prometheus metrics
that can be scraped, dashboarded, and alerted on with the rest of the user's
observability stack.

## Non-goals

The following are explicitly out of scope for this change. Each can be a
follow-up if demand emerges:

- Authentication on `/metrics`. Operators restrict the endpoint at their
  reverse proxy. Documented in the configuration page.
- OpenTelemetry / OTLP export. Sablier already pulls `otel/metric`
  transitively but this proposal uses the Prometheus client directly.
- Per-strategy histograms (separate ready-time histogram for blocking vs
  dynamic). The `instance` label is sufficient; strategy-level views can be
  built in PromQL by joining with the request counter.
- Tracing spans.
- Persisting active-instance state across Sablier restarts. With the
  in-memory store, sessions are lost on restart and gauges correctly reset
  to 0. With the Valkey store, sessions survive but the metrics-side
  active-instance set starts empty and repopulates as requests come in.
  Reconstructing it from Valkey on startup is a possible follow-up but
  requires expanding the `Store` interface.
- Splitting `/metrics` onto a separate listener / port. Single listener for
  now; can be added later.

## User-facing surface

### Configuration

A new substruct under `server`:

```yaml
server:
  port: 10000
  base-path: /
  metrics:
    enabled: true   # default: false
```

Equivalent CLI flag: `--server.metrics.enabled`. Equivalent env var:
`SERVER_METRICS_ENABLED` (Viper auto-binding, matches existing patterns).

### Endpoint

`GET <base-path>/metrics`. Served by the same gin server that already
handles `/health` and `/api/...`. Registered only when
`server.metrics.enabled = true`. Returns 404 when disabled.

The endpoint uses `promhttp.HandlerFor` against a per-process registry,
wrapped with `gin.WrapH`.

## Metrics catalog

| Name | Type | Labels | Source |
|------|------|--------|--------|
| `sablier_group_locked` | gauge | `group` | Lazy collector. `1` if any instance in the group has an active session, else `0`. One series per known group, including groups with no active sessions. |
| `sablier_group_active_instances` | gauge | `group` | Lazy collector. Number of instances in the group that currently have an active session. |
| `sablier_instance_start_duration_seconds` | histogram | `instance` | Observed when `provider.InstanceStart` returns successfully (in `requestStart`'s goroutine). |
| `sablier_instance_ready_duration_seconds` | histogram | `instance` | Observed in `InstanceRequest` when a previously-not-ready instance transitions to `Ready`. Start time is recorded on the first not-ready observation for that instance. |
| `sablier_session_requests_total` | counter | `strategy` (`dynamic`\|`blocking`), `target` (`names`\|`group`) | Incremented at the top of each strategy handler. |
| `sablier_instance_start_failures_total` | counter | `instance` | Incremented in `requestStart`'s goroutine when `provider.InstanceStart` returns an error. |
| `sablier_instance_stops_total` | counter | `instance`, `reason` (`expired`\|`unregistered`) | Incremented in the store's expiration callback (alongside the existing `provider.InstanceStop` call) and in `StopAllUnregisteredInstances`. |
| Go runtime + process collectors | (default) | (default) | `prometheus.NewGoCollector()` + `prometheus.NewProcessCollector()` registered on the same registry. |

### Histogram buckets

Both duration histograms use explicit buckets sized for container start
times (the prometheus default `[5ms, 10s]` set is wrong for this domain):

```
[0.1, 0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300]   # seconds
```

100 ms to 5 minutes, with denser resolution at the lower end.

### Cardinality

The `instance` label is per-instance. Sablier deployments typically have
tens of instances, occasionally low hundreds. Total series cost is roughly
`2 histograms × 11 buckets + counters ≈ 25 series per instance`. At 100
instances that is ~2,500 series, comfortably within Prometheus norms.

The `group` gauges produce two series per known group regardless of
activity — bounded by the number of configured groups.

## Architecture

### New package: `pkg/metrics`

The new package owns everything Prometheus-related:

- `type Recorder interface` — the surface that Sablier core and the API
  handlers call into when an event happens. One method per event:
  - `RecordSessionRequest(strategy, target string)`
  - `RecordInstanceStartEnd(instance string, dur time.Duration)`
  - `RecordInstanceStartFailure(instance string)`
  - `RecordReadyWaitBegin(instance string)`
  - `RecordReadyWaitEnd(instance string)`
  - `RecordActiveInstance(instance string)`
  - `RecordInactiveInstance(instance string)`
  - `RecordInstanceStop(instance, reason string)`
- `type Noop struct{}` — zero-overhead default returned when metrics are
  disabled. Used unconditionally so call sites stay branch-free.
- `type PromRecorder struct { ... }` — real implementation backed by a
  `*prometheus.Registry`. Owns its own state: `activeInstances`
  (`map[string]struct{}`) and `readyWait` (`map[string]time.Time`), each
  with its own mutex.
- `func NewHandler(r *PromRecorder) http.Handler` — produced from
  `promhttp.HandlerFor(r.Registry, promhttp.HandlerOpts{})`.
- `type GroupsProvider interface { Groups() map[string][]string }` — a
  one-method interface implemented by `*Sablier` (returns a snapshot under
  `groupsMu.RLock()`).
- `type GroupLockCollector` — a custom `prometheus.Collector` that emits
  `sablier_group_locked` and `sablier_group_active_instances` lazily at
  scrape time. Holds a `GroupsProvider` and a reference to the recorder's
  `activeInstances` set.

### Why a Recorder interface

- Isolates Sablier core from Prometheus types.
- Lets `Noop` be the default with no per-call branches in callers.
- Makes unit tests trivial — substitute a fake recorder that records calls.
- Lets the metrics package own all of its mutable state without leaking
  into `Sablier`.

### How "locked / active per group" is computed

A `GroupLockCollector.Collect` call (invoked at scrape time):

1. Reads `Sablier.groups` via the `GroupsProvider` (snapshot under `groupsMu.RLock`).
2. Reads the recorder's `activeInstances` set under its own lock.
3. For each known group, counts members present in `activeInstances`.
4. Emits a `sablier_group_active_instances{group=G} N` sample.
5. Emits `sablier_group_locked{group=G} 1` if `N > 0`, else `0`.

Costs: O(groups × instances-per-group) at scrape time, all in-memory map
lookups. No store calls per scrape (important for the Valkey backend).

`activeInstances` is updated synchronously by the same code paths that
manipulate sessions:

- `Sablier.requestStart` calls `RecordActiveInstance(name)` when it
  registers a new pending start.
- The `OnInstanceExpired` store callback calls
  `RecordInactiveInstance(name)` next to the existing
  `provider.InstanceStop` call.

### How end-to-end ready time is measured

`PromRecorder.readyWait` (`map[string]time.Time`) is keyed by instance:

- `RecordReadyWaitBegin(name)` — sets `readyWait[name] = time.Now()` only
  if absent. Idempotent on repeated polls (every blocking-strategy refresh
  re-enters `InstanceRequest`).
- `RecordReadyWaitEnd(name)` — observes the histogram with
  `time.Since(readyWait[name])` and deletes the entry, but only if an
  entry exists. Calling `End` without a prior `Begin` is a silent no-op
  (defensive against persistent-store restarts and other edge cases).

Ready transition is detected in `InstanceRequest`: after the
`provider.InstanceInspect` call, if the resulting state is `Ready`, call
`RecordReadyWaitEnd(name)`.

`RecordReadyWaitBegin` is called inside `requestStart` **before**
dispatching the goroutine — otherwise a fast provider could finish the
start and emit the End before Begin runs.

### Provider start duration

Measured by wrapping the goroutine body in `requestStart`:

```go
start := time.Now()
err := s.provider.InstanceStart(startCtx, name)
if err != nil {
    s.metrics.RecordInstanceStartFailure(name)
    ...
} else {
    s.metrics.RecordInstanceStartEnd(name, time.Since(start))
}
```

The histogram is observed only on success — failures populate the
failure counter instead, to keep the latency distribution clean.

## Call-site changes

| File | Change |
|------|--------|
| `pkg/sablier/sablier.go` | Add `metrics metrics.Recorder` field, default `Noop{}`. Add `WithMetrics(r metrics.Recorder)` setter. |
| `pkg/sablier/instance_request.go` | In `requestStart`: `RecordReadyWaitBegin` + `RecordActiveInstance` before goroutine; time `provider.InstanceStart`; `RecordInstanceStartEnd` (and `RecordInstanceStartFailure` on error). In `InstanceRequest`: call `RecordReadyWaitEnd` on the ready transition. |
| `pkg/sablier/instance_expired.go` | In `OnInstanceExpired` callback: `RecordInstanceStop(name, "expired")` and `RecordInactiveInstance(name)` next to the existing `provider.InstanceStop` call. |
| `pkg/sablier/autostop.go` | In `StopAllUnregisteredInstances`: `RecordInstanceStop(name, "unregistered")` per instance. |
| `internal/api/api.go` | Add `Metrics metrics.Recorder` field on `ServeStrategy`. |
| `internal/api/start_dynamic.go` | At top of handler: `s.Metrics.RecordSessionRequest("dynamic", target)` where `target` is `"names"` or `"group"`. |
| `internal/api/start_blocking.go` | Same for `"blocking"`. |
| `internal/server/routes.go` | Register `<base-path>/metrics` only when `s.Metrics` is non-Noop. Handler from `metrics.NewHandler(rec)`, wrapped with `gin.WrapH`. |
| `pkg/sabliercmd/start.go` | Read `conf.Server.Metrics.Enabled`. If true: build `PromRecorder`, register `GroupLockCollector` (passing the `*Sablier` as `GroupsProvider`), wire into `sablier.New` and `ServeStrategy`. If false: pass `Noop{}`. |
| `pkg/config/server.go` | Add `MetricsConfig{ Enabled bool }` substruct on `Server`. |
| `sablier.sample.yaml` | Document new option. |
| `docs/configuration.md` | Document new option, endpoint location, security note. |
| `go.mod` | Promote `github.com/prometheus/client_golang` from transitive to direct dependency. |

## Testing

### `pkg/metrics` unit tests

- `Noop` recorder: no panics, all methods return cleanly.
- `PromRecorder`: each `Record*` method updates the right metric. Use
  `prometheus/testutil.ToFloat64` and `testutil.CollectAndCompare`.
- `GroupLockCollector`: feed it a fake `groupsProvider` and a manually
  populated active set; scrape; assert correct series, including
  zero-value series for known-but-empty groups.
- Histogram observation: `RecordInstanceStartEnd` produces a sample in
  the expected bucket.
- `RecordReadyWaitEnd` without `RecordReadyWaitBegin` is a no-op.

### `pkg/sablier` unit tests

- Add a `fakeRecorder` that captures calls into a slice; pass it via
  `WithMetrics`.
- `requestStart` calls `RecordReadyWaitBegin` + `RecordActiveInstance`
  before the goroutine runs; calls `RecordInstanceStartEnd` on success
  and `RecordInstanceStartFailure` (only) on failure.
- `InstanceRequest` calls `RecordReadyWaitEnd` exactly once on the
  not-ready→ready transition.

### `internal/server` integration test

- Start a gin server with metrics enabled. Hit `<base-path>/metrics`.
  Assert response is `text/plain` (Prometheus exposition format) and
  contains the expected metric names with `# HELP` / `# TYPE` lines.
- Second test with metrics disabled: `/metrics` returns 404.

### Out of scope

- The existing testcontainers-based provider tests do not need to change.
  Real provider start times will naturally observe the histograms but no
  assertions are added on real durations (would be flaky).

## Security

The endpoint exposes:

- Group names and per-group lock state.
- Instance names (in metric labels).
- Sablier process internals (heap, goroutines, file descriptors).
- Counters and durations.

These are operator-level details. Documentation will state that
`/metrics` is intended for trusted observability stacks and should be
restricted at the reverse proxy when Sablier is fronted on an untrusted
network. This matches the existing posture of `/health` (no auth).

## Compatibility and migration

- Purely additive. No existing config or HTTP contract changes.
- Default is opt-out (disabled). Existing deployments see no change in
  behavior or response surface.
- `prometheus/client_golang` is already a transitive dependency, so no
  meaningful change to the dependency footprint.
