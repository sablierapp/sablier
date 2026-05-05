# Prometheus `/metrics` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in Prometheus `/metrics` endpoint to Sablier exposing group lock state, per-instance warmup time, session counters, and Go runtime/process collectors.

**Architecture:** New `pkg/metrics` package owns a `Recorder` interface (`Noop` default + `PromRecorder` real impl) plus a custom `GroupLockCollector` that emits group gauges lazily at scrape time. Sablier core, API handlers, and the store-expiration callback call into the recorder at well-defined event points. The endpoint is wired up in `start.go` only when `server.metrics.enabled = true`.

**Tech Stack:** Go 1.26, gin, `github.com/prometheus/client_golang` (already a transitive dep — promoted to direct), `viper` for config, `gomock`/`gotest.tools/v3` for tests.

**Spec:** [`docs/proposals/2026-05-05-prometheus-metrics.md`](./2026-05-05-prometheus-metrics.md) — read before starting.

---

## File map

| Path | Action | Responsibility |
|------|--------|----------------|
| `pkg/metrics/recorder.go` | create | `Recorder` interface + `Noop` zero-overhead implementation |
| `pkg/metrics/prom.go` | create | `PromRecorder` struct, registry construction, all Record* methods, internal `activeInstances` + `readyWait` state |
| `pkg/metrics/collector.go` | create | `GroupsProvider` interface + `GroupLockCollector` (custom Collector for group gauges) |
| `pkg/metrics/handler.go` | create | `NewHandler` factory wrapping `promhttp.HandlerFor` |
| `pkg/metrics/recorder_test.go` | create | Unit tests for Noop |
| `pkg/metrics/prom_test.go` | create | Unit tests for PromRecorder counters/histograms/state |
| `pkg/metrics/collector_test.go` | create | Unit tests for GroupLockCollector |
| `pkg/metrics/handler_test.go` | create | Unit test for HTTP handler |
| `pkg/config/server.go` | modify | Add `MetricsConfig{ Enabled bool }` substruct |
| `pkg/sablier/sablier.go` | modify | Add `metrics metrics.Recorder` field, `WithMetrics` setter, `Groups()` accessor |
| `pkg/sablier/instance_request.go` | modify | Wire metrics calls in `requestStart` and `InstanceRequest` |
| `pkg/sablier/instance_request_test.go` | modify | Add `fakeRecorder` and tests verifying metric calls |
| `pkg/sablier/instance_expired.go` | modify | Add recorder parameter, call `RecordInstanceStop` + `RecordInactiveInstance` |
| `pkg/sablier/autostop.go` | modify | Call `RecordInstanceStop("unregistered")` per stopped instance |
| `internal/api/api.go` | modify | Add `Metrics metrics.Recorder` field on `ServeStrategy` |
| `internal/api/start_dynamic.go` | modify | Increment `sablier_session_requests_total` |
| `internal/api/start_blocking.go` | modify | Increment `sablier_session_requests_total` |
| `internal/server/routes.go` | modify | Register `<base-path>/metrics` when recorder is real |
| `internal/server/metrics_test.go` | create | Integration test for the endpoint |
| `pkg/sabliercmd/start.go` | modify | Build `PromRecorder` when enabled, register collector, pass everywhere |
| `sablier.sample.yaml` | modify | Document `server.metrics.enabled` |
| `docs/configuration.md` | modify | Document the new option, endpoint location, security note |
| `go.mod` | modify | Promote `github.com/prometheus/client_golang` to direct dep (`go mod tidy`) |

---

## Task 1: Bootstrap — verify branch and confirm prom dep is available

**Files:** none (read-only)

- [ ] **Step 1: Confirm we're on the feature branch**

Run: `git -C /Users/shorty/Src/github.com/sablierapp/sablier branch --show-current`
Expected: `feat/prometheus-metrics`

- [ ] **Step 2: Confirm prometheus client is reachable as a transitive dep**

Run: `grep prometheus/client_golang /Users/shorty/Src/github.com/sablierapp/sablier/go.sum | head -1`
Expected: a line containing `github.com/prometheus/client_golang v1.22.0` (or newer).

- [ ] **Step 3: Run the existing test suite to confirm baseline is green**

Run: `go test ./pkg/sablier/... ./internal/...`
Expected: all PASS. If anything is already broken, stop and surface — do not start adding code on a red baseline.

No commit in this task.

---

## Task 2: Add `MetricsConfig` to server config

**Files:**
- Modify: `pkg/config/server.go`

- [ ] **Step 1: Edit `pkg/config/server.go` to add the substruct**

Replace the current contents with:

```go
package config

type Server struct {
	Port     int           `mapstructure:"PORT" yaml:"port" default:"10000"`
	BasePath string        `mapstructure:"BASE_PATH" yaml:"basePath" default:"/"`
	Metrics  MetricsConfig `mapstructure:"METRICS" yaml:"metrics"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"ENABLED" yaml:"enabled" default:"false"`
}

func NewServerConfig() Server {
	return Server{
		Port:     10000,
		BasePath: "/",
		Metrics:  MetricsConfig{Enabled: false},
	}
}
```

- [ ] **Step 2: Verify the package still builds**

Run: `go build ./pkg/config/...`
Expected: clean build, no output.

- [ ] **Step 3: Verify the existing config tests still pass**

Run: `go test ./pkg/sabliercmd/...`
Expected: PASS. If `testdata/config_default.json` fails because the JSON snapshot now includes the new `Metrics` block, update the snapshot file to include `"Metrics": {"Enabled": false}` under `"Server"` in the appropriate position (read the snapshot, find the `Server` object, add the field). Commit the snapshot change as part of this task.

- [ ] **Step 4: Commit**

```bash
git add pkg/config/server.go pkg/sabliercmd/testdata/
git commit -m "feat(config): add server.metrics.enabled (default false)"
```

---

## Task 3: Create `pkg/metrics` with `Recorder` interface and `Noop`

**Files:**
- Create: `pkg/metrics/recorder.go`
- Create: `pkg/metrics/recorder_test.go`

- [ ] **Step 1: Write the failing test first**

Create `pkg/metrics/recorder_test.go`:

```go
package metrics_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestNoopRecorderImplementsAllMethods(t *testing.T) {
	var r metrics.Recorder = metrics.Noop{}

	// All methods must be callable and return without panic.
	r.RecordSessionRequest("dynamic", "names")
	r.RecordInstanceStartEnd("nginx", 250*time.Millisecond)
	r.RecordInstanceStartFailure("nginx")
	r.RecordReadyWaitBegin("nginx")
	r.RecordReadyWaitEnd("nginx")
	r.RecordActiveInstance("nginx")
	r.RecordInactiveInstance("nginx")
	r.RecordInstanceStop("nginx", "expired")

	// Sanity: errors.Is on nil error returned from no-op behavior is irrelevant; just touch the package.
	_ = errors.Is(nil, nil)
}
```

- [ ] **Step 2: Run the test to verify it fails (no package yet)**

Run: `go test ./pkg/metrics/...`
Expected: FAIL with "no Go files in" or "package metrics is not in std".

- [ ] **Step 3: Create the package and the interface + Noop**

Create `pkg/metrics/recorder.go`:

```go
// Package metrics provides Prometheus instrumentation for Sablier.
package metrics

import "time"

// Recorder is the surface that Sablier core and the API handlers call into
// when an event happens. The Noop implementation is used when metrics are
// disabled; PromRecorder is the real Prometheus-backed implementation.
type Recorder interface {
	RecordSessionRequest(strategy, target string)
	RecordInstanceStartEnd(instance string, dur time.Duration)
	RecordInstanceStartFailure(instance string)
	RecordReadyWaitBegin(instance string)
	RecordReadyWaitEnd(instance string)
	RecordActiveInstance(instance string)
	RecordInactiveInstance(instance string)
	RecordInstanceStop(instance, reason string)
}

// Noop is the zero-overhead default recorder. It is used when metrics are
// disabled so call sites can call methods unconditionally without branching.
type Noop struct{}

func (Noop) RecordSessionRequest(string, string)          {}
func (Noop) RecordInstanceStartEnd(string, time.Duration) {}
func (Noop) RecordInstanceStartFailure(string)            {}
func (Noop) RecordReadyWaitBegin(string)                  {}
func (Noop) RecordReadyWaitEnd(string)                    {}
func (Noop) RecordActiveInstance(string)                  {}
func (Noop) RecordInactiveInstance(string)                {}
func (Noop) RecordInstanceStop(string, string)            {}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/
git commit -m "feat(metrics): add Recorder interface and Noop implementation"
```

---

## Task 4: `PromRecorder` skeleton with registry, Go and process collectors

**Files:**
- Create: `pkg/metrics/prom.go`
- Create: `pkg/metrics/prom_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/metrics/prom_test.go`:

```go
package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestNewPromRecorderRegistersGoAndProcessCollectors(t *testing.T) {
	r := metrics.NewPromRecorder()
	if r == nil {
		t.Fatal("NewPromRecorder returned nil")
	}

	reg := r.Registry()
	if reg == nil {
		t.Fatal("Registry() returned nil")
	}

	// Gather metric families and assert standard collectors are present.
	families, err := reg.(*prometheus.Registry).Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}

	// At minimum, expect the well-known go_goroutines metric from the Go collector
	// and process_start_time_seconds from the process collector.
	if !names["go_goroutines"] {
		t.Errorf("expected go_goroutines metric to be registered, got: %v", keys(names))
	}
	if !names["process_start_time_seconds"] {
		t.Errorf("expected process_start_time_seconds metric to be registered, got: %v", keys(names))
	}

	// Sanity: our custom counter is also registered (even before being incremented,
	// counters with no labels appear in Gather; counter vectors only after first observation,
	// so this assertion is loose).
	_ = testutil.CollectAndCount
	_ = strings.Builder{}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `metrics.NewPromRecorder undefined`.

- [ ] **Step 3: Implement `PromRecorder` skeleton**

Create `pkg/metrics/prom.go`:

```go
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// histogramBuckets covers container start times: 100ms to 5min.
var histogramBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300}

// PromRecorder is the Prometheus-backed Recorder. It owns its own metric
// vectors and the small amount of internal state required for the lazy group
// gauges and the ready-wait histogram.
type PromRecorder struct {
	registry *prometheus.Registry

	sessionRequests       *prometheus.CounterVec
	instanceStartFailures *prometheus.CounterVec
	instanceStops         *prometheus.CounterVec
	instanceStartDuration *prometheus.HistogramVec
	instanceReadyDuration *prometheus.HistogramVec

	activeMu       sync.RWMutex
	activeInstances map[string]struct{}

	readyMu   sync.Mutex
	readyWait map[string]time.Time
}

// NewPromRecorder constructs a PromRecorder, registers all metric vectors and
// the standard Go and process collectors on a fresh registry.
func NewPromRecorder() *PromRecorder {
	reg := prometheus.NewRegistry()

	r := &PromRecorder{
		registry:        reg,
		activeInstances: make(map[string]struct{}),
		readyWait:       make(map[string]time.Time),
	}

	r.sessionRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sablier_session_requests_total",
			Help: "Total number of session requests received, by strategy and target.",
		},
		[]string{"strategy", "target"},
	)
	r.instanceStartFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sablier_instance_start_failures_total",
			Help: "Total number of provider InstanceStart failures, by instance.",
		},
		[]string{"instance"},
	)
	r.instanceStops = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sablier_instance_stops_total",
			Help: "Total number of instance stops, by instance and reason.",
		},
		[]string{"instance", "reason"},
	)
	r.instanceStartDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sablier_instance_start_duration_seconds",
			Help:    "Duration of provider.InstanceStart calls (seconds), only successful starts.",
			Buckets: histogramBuckets,
		},
		[]string{"instance"},
	)
	r.instanceReadyDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sablier_instance_ready_duration_seconds",
			Help:    "End-to-end wall time from first not-ready observation to ready (seconds).",
			Buckets: histogramBuckets,
		},
		[]string{"instance"},
	)

	reg.MustRegister(
		r.sessionRequests,
		r.instanceStartFailures,
		r.instanceStops,
		r.instanceStartDuration,
		r.instanceReadyDuration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return r
}

// Registry returns the underlying registry so the HTTP handler and external
// custom collectors (e.g. GroupLockCollector) can be wired in.
func (r *PromRecorder) Registry() prometheus.Registerer {
	// Returns a Registerer (not *Registry) to keep the surface narrow for callers.
	return r.registry
}

// Gather is exposed to test helpers.
func (r *PromRecorder) gather() ([]*prometheus.MetricFamily, error) {
	// Defensive helper for tests; the registry is package-internal.
	mfs, err := r.registry.Gather()
	if err != nil {
		return nil, err
	}
	out := make([]*prometheus.MetricFamily, len(mfs))
	for i := range mfs {
		out[i] = mfs[i]
	}
	return out, nil
}
```

- [ ] **Step 4: Adjust the test — `Registry()` returns `prometheus.Registerer`**

The skeleton above returns a `Registerer` from `Registry()`. Update `prom_test.go` to type-assert before calling `Gather`:

```go
	reg := r.Registry()
	if reg == nil {
		t.Fatal("Registry() returned nil")
	}

	families, err := reg.(*prometheus.Registry).Gather()
```

This is already in the test as written — keep it as is.

- [ ] **Step 5: Run `go mod tidy` to promote the dep**

Run: `go mod tidy`
Expected: `go.mod` now lists `github.com/prometheus/client_golang` and `github.com/prometheus/client_model` (transitively) under direct `require`. No errors.

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum pkg/metrics/prom.go pkg/metrics/prom_test.go
git commit -m "feat(metrics): add PromRecorder skeleton with Go and process collectors"
```

---

## Task 5: Counter `Record*` methods on `PromRecorder`

**Files:**
- Modify: `pkg/metrics/prom.go`
- Modify: `pkg/metrics/prom_test.go`

- [ ] **Step 1: Add the failing tests**

Append to `pkg/metrics/prom_test.go`:

```go
func TestPromRecorder_Counters(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordSessionRequest("dynamic", "names")
	r.RecordSessionRequest("dynamic", "names")
	r.RecordSessionRequest("blocking", "group")
	r.RecordInstanceStartFailure("nginx")
	r.RecordInstanceStop("nginx", "expired")
	r.RecordInstanceStop("nginx", "unregistered")

	mustCounter(t, r, "sablier_session_requests_total", map[string]string{"strategy": "dynamic", "target": "names"}, 2)
	mustCounter(t, r, "sablier_session_requests_total", map[string]string{"strategy": "blocking", "target": "group"}, 1)
	mustCounter(t, r, "sablier_instance_start_failures_total", map[string]string{"instance": "nginx"}, 1)
	mustCounter(t, r, "sablier_instance_stops_total", map[string]string{"instance": "nginx", "reason": "expired"}, 1)
	mustCounter(t, r, "sablier_instance_stops_total", map[string]string{"instance": "nginx", "reason": "unregistered"}, 1)
}

// mustCounter reads the named counter with the given labels and asserts the value.
func mustCounter(t *testing.T, r *metrics.PromRecorder, name string, labels map[string]string, want float64) {
	t.Helper()
	mfs, err := r.Registry().(*prometheus.Registry).Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if !labelsMatch(m.GetLabel(), labels) {
				continue
			}
			got := m.GetCounter().GetValue()
			if got != want {
				t.Errorf("%s%v = %v, want %v", name, labels, got, want)
			}
			return
		}
	}
	t.Errorf("counter %s%v not found", name, labels)
}

func labelsMatch(got []*io_prometheus_client.LabelPair, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, lp := range got {
		if want[lp.GetName()] != lp.GetValue() {
			return false
		}
	}
	return true
}
```

Add the import for `io_prometheus_client "github.com/prometheus/client_model/go"` at the top of the test file.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `r.RecordSessionRequest undefined` (or similar).

- [ ] **Step 3: Add the counter methods to `prom.go`**

Append to `pkg/metrics/prom.go`:

```go
func (r *PromRecorder) RecordSessionRequest(strategy, target string) {
	r.sessionRequests.WithLabelValues(strategy, target).Inc()
}

func (r *PromRecorder) RecordInstanceStartFailure(instance string) {
	r.instanceStartFailures.WithLabelValues(instance).Inc()
}

func (r *PromRecorder) RecordInstanceStop(instance, reason string) {
	r.instanceStops.WithLabelValues(instance, reason).Inc()
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/prom.go pkg/metrics/prom_test.go
git commit -m "feat(metrics): record session requests, start failures, stops"
```

---

## Task 6: Histogram `RecordInstanceStartEnd` on `PromRecorder`

**Files:**
- Modify: `pkg/metrics/prom.go`
- Modify: `pkg/metrics/prom_test.go`

- [ ] **Step 1: Add the failing test**

Append to `pkg/metrics/prom_test.go`:

```go
func TestPromRecorder_InstanceStartDuration(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordInstanceStartEnd("nginx", 750*time.Millisecond)
	r.RecordInstanceStartEnd("nginx", 3*time.Second)
	r.RecordInstanceStartEnd("redis", 30*time.Second)

	mustHistogramCount(t, r, "sablier_instance_start_duration_seconds", map[string]string{"instance": "nginx"}, 2)
	mustHistogramCount(t, r, "sablier_instance_start_duration_seconds", map[string]string{"instance": "redis"}, 1)
}

func mustHistogramCount(t *testing.T, r *metrics.PromRecorder, name string, labels map[string]string, want uint64) {
	t.Helper()
	mfs, err := r.Registry().(*prometheus.Registry).Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if !labelsMatch(m.GetLabel(), labels) {
				continue
			}
			got := m.GetHistogram().GetSampleCount()
			if got != want {
				t.Errorf("%s%v sample count = %d, want %d", name, labels, got, want)
			}
			return
		}
	}
	t.Errorf("histogram %s%v not found", name, labels)
}
```

Make sure `time` is imported in the test file (it already is from Task 3's test).

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `r.RecordInstanceStartEnd undefined`.

- [ ] **Step 3: Add the method**

Append to `pkg/metrics/prom.go`:

```go
func (r *PromRecorder) RecordInstanceStartEnd(instance string, dur time.Duration) {
	r.instanceStartDuration.WithLabelValues(instance).Observe(dur.Seconds())
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/prom.go pkg/metrics/prom_test.go
git commit -m "feat(metrics): observe instance start duration histogram"
```

---

## Task 7: Ready-wait Begin/End with stateful timing

**Files:**
- Modify: `pkg/metrics/prom.go`
- Modify: `pkg/metrics/prom_test.go`

- [ ] **Step 1: Add the failing tests**

Append to `pkg/metrics/prom_test.go`:

```go
func TestPromRecorder_ReadyWait_BeginThenEnd(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordReadyWaitBegin("nginx")
	r.RecordReadyWaitEnd("nginx")

	mustHistogramCount(t, r, "sablier_instance_ready_duration_seconds", map[string]string{"instance": "nginx"}, 1)
}

func TestPromRecorder_ReadyWait_EndWithoutBeginIsNoop(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordReadyWaitEnd("nginx") // no Begin first

	// Histogram should have no samples for nginx — assert by attempting to find the series and treating absence as count==0.
	mfs, err := r.Registry().(*prometheus.Registry).Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "sablier_instance_ready_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m.GetLabel(), map[string]string{"instance": "nginx"}) {
				t.Errorf("expected no samples but found %d", m.GetHistogram().GetSampleCount())
			}
		}
	}
}

func TestPromRecorder_ReadyWait_BeginIsIdempotent(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordReadyWaitBegin("nginx")
	first := readyWaitTimestamp(t, r, "nginx")
	time.Sleep(20 * time.Millisecond)
	r.RecordReadyWaitBegin("nginx") // must not reset
	second := readyWaitTimestamp(t, r, "nginx")

	if !first.Equal(second) {
		t.Errorf("RecordReadyWaitBegin reset the timestamp: %v -> %v", first, second)
	}
}

// readyWaitTimestamp reads the internal map via a test-only helper exposed below.
func readyWaitTimestamp(t *testing.T, r *metrics.PromRecorder, instance string) time.Time {
	t.Helper()
	ts, ok := r.ReadyWaitStartedAt(instance)
	if !ok {
		t.Fatalf("no ready-wait timestamp for %q", instance)
	}
	return ts
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `r.RecordReadyWaitBegin undefined`.

- [ ] **Step 3: Implement Begin/End plus the test-only accessor**

Append to `pkg/metrics/prom.go`:

```go
func (r *PromRecorder) RecordReadyWaitBegin(instance string) {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()
	if _, exists := r.readyWait[instance]; exists {
		return // idempotent — first observation wins
	}
	r.readyWait[instance] = time.Now()
}

func (r *PromRecorder) RecordReadyWaitEnd(instance string) {
	r.readyMu.Lock()
	start, exists := r.readyWait[instance]
	if !exists {
		r.readyMu.Unlock()
		return
	}
	delete(r.readyWait, instance)
	r.readyMu.Unlock()

	r.instanceReadyDuration.WithLabelValues(instance).Observe(time.Since(start).Seconds())
}

// ReadyWaitStartedAt returns the timestamp recorded for an instance's pending
// ready transition, if any. Test-only helper; not part of the Recorder
// interface.
func (r *PromRecorder) ReadyWaitStartedAt(instance string) (time.Time, bool) {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()
	t, ok := r.readyWait[instance]
	return t, ok
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/prom.go pkg/metrics/prom_test.go
git commit -m "feat(metrics): record end-to-end ready wait time per instance"
```

---

## Task 8: Active instance tracking + accessor

**Files:**
- Modify: `pkg/metrics/prom.go`
- Modify: `pkg/metrics/prom_test.go`

- [ ] **Step 1: Add the failing tests**

Append to `pkg/metrics/prom_test.go`:

```go
func TestPromRecorder_ActiveInstances(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordActiveInstance("nginx")
	r.RecordActiveInstance("redis")
	r.RecordActiveInstance("nginx") // idempotent

	got := r.SnapshotActiveInstances()
	want := map[string]bool{"nginx": true, "redis": true}
	if len(got) != len(want) {
		t.Fatalf("active set size = %d, want %d (%v)", len(got), len(want), got)
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("expected %q in active set, got %v", k, got)
		}
	}

	r.RecordInactiveInstance("nginx")
	got = r.SnapshotActiveInstances()
	if _, ok := got["nginx"]; ok {
		t.Errorf("expected nginx removed, got %v", got)
	}
	if _, ok := got["redis"]; !ok {
		t.Errorf("expected redis still present, got %v", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `r.RecordActiveInstance undefined` and `r.SnapshotActiveInstances undefined`.

- [ ] **Step 3: Implement**

Append to `pkg/metrics/prom.go`:

```go
func (r *PromRecorder) RecordActiveInstance(instance string) {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	r.activeInstances[instance] = struct{}{}
}

func (r *PromRecorder) RecordInactiveInstance(instance string) {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	delete(r.activeInstances, instance)
}

// SnapshotActiveInstances returns a copy of the current active set. Used by
// the GroupLockCollector at scrape time and by tests.
func (r *PromRecorder) SnapshotActiveInstances() map[string]struct{} {
	r.activeMu.RLock()
	defer r.activeMu.RUnlock()
	out := make(map[string]struct{}, len(r.activeInstances))
	for k := range r.activeInstances {
		out[k] = struct{}{}
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/prom.go pkg/metrics/prom_test.go
git commit -m "feat(metrics): track active instances for group lock gauges"
```

---

## Task 9: `GroupLockCollector` and `GroupsProvider` interface

**Files:**
- Create: `pkg/metrics/collector.go`
- Create: `pkg/metrics/collector_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/metrics/collector_test.go`:

```go
package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sablierapp/sablier/pkg/metrics"
)

type fakeGroupsProvider struct {
	groups map[string][]string
}

func (f fakeGroupsProvider) Groups() map[string][]string {
	out := make(map[string][]string, len(f.groups))
	for k, v := range f.groups {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func TestGroupLockCollector_EmitsZeroAndNonZero(t *testing.T) {
	r := metrics.NewPromRecorder()
	r.RecordActiveInstance("web1")
	// "web2" intentionally not active

	gp := fakeGroupsProvider{groups: map[string][]string{
		"web":   {"web1", "web2"},
		"empty": {"none1", "none2"},
	}}

	c := metrics.NewGroupLockCollector(gp, r)
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	want := `
# HELP sablier_group_active_instances Number of instances in the group with an active session.
# TYPE sablier_group_active_instances gauge
sablier_group_active_instances{group="empty"} 0
sablier_group_active_instances{group="web"} 1
# HELP sablier_group_locked Whether the group has at least one instance with an active session (1) or not (0).
# TYPE sablier_group_locked gauge
sablier_group_locked{group="empty"} 0
sablier_group_locked{group="web"} 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(want),
		"sablier_group_active_instances", "sablier_group_locked"); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}

func TestGroupLockCollector_NoGroupsEmitsNothing(t *testing.T) {
	r := metrics.NewPromRecorder()
	gp := fakeGroupsProvider{groups: map[string][]string{}}

	c := metrics.NewGroupLockCollector(gp, r)
	got := testutil.CollectAndCount(c, "sablier_group_locked", "sablier_group_active_instances")
	if got != 0 {
		t.Errorf("expected 0 series with no groups, got %d", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `metrics.NewGroupLockCollector undefined`.

- [ ] **Step 3: Implement the collector**

Create `pkg/metrics/collector.go`:

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

// GroupsProvider exposes the current configured groups (group name -> instance names).
// Implemented by *sablier.Sablier under its own mutex.
type GroupsProvider interface {
	Groups() map[string][]string
}

// activeSetProvider is the subset of PromRecorder used by the collector at
// scrape time. Decoupled to keep the collector testable.
type activeSetProvider interface {
	SnapshotActiveInstances() map[string]struct{}
}

// GroupLockCollector emits the two group-level gauges lazily at scrape time:
// sablier_group_locked (binary) and sablier_group_active_instances (count).
type GroupLockCollector struct {
	groups GroupsProvider
	active activeSetProvider

	lockedDesc *prometheus.Desc
	countDesc  *prometheus.Desc
}

// NewGroupLockCollector wires a GroupsProvider and a PromRecorder (or any
// activeSetProvider) into a Collector that can be registered on the same
// registry as the rest of the metrics.
func NewGroupLockCollector(groups GroupsProvider, active activeSetProvider) *GroupLockCollector {
	return &GroupLockCollector{
		groups: groups,
		active: active,
		lockedDesc: prometheus.NewDesc(
			"sablier_group_locked",
			"Whether the group has at least one instance with an active session (1) or not (0).",
			[]string{"group"}, nil,
		),
		countDesc: prometheus.NewDesc(
			"sablier_group_active_instances",
			"Number of instances in the group with an active session.",
			[]string{"group"}, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *GroupLockCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.lockedDesc
	ch <- c.countDesc
}

// Collect implements prometheus.Collector. Walks the current groups and the
// active-instance snapshot, emitting one row per known group for both gauges.
func (c *GroupLockCollector) Collect(ch chan<- prometheus.Metric) {
	groups := c.groups.Groups()
	active := c.active.SnapshotActiveInstances()

	for group, members := range groups {
		count := 0
		for _, m := range members {
			if _, ok := active[m]; ok {
				count++
			}
		}
		locked := 0.0
		if count > 0 {
			locked = 1.0
		}
		ch <- prometheus.MustNewConstMetric(c.lockedDesc, prometheus.GaugeValue, locked, group)
		ch <- prometheus.MustNewConstMetric(c.countDesc, prometheus.GaugeValue, float64(count), group)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/collector.go pkg/metrics/collector_test.go
git commit -m "feat(metrics): GroupLockCollector emits per-group gauges"
```

---

## Task 10: HTTP handler factory

**Files:**
- Create: `pkg/metrics/handler.go`
- Create: `pkg/metrics/handler_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/metrics/handler_test.go`:

```go
package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestNewHandler_ServesPrometheusExposition(t *testing.T) {
	r := metrics.NewPromRecorder()
	r.RecordSessionRequest("dynamic", "names")

	srv := httptest.NewServer(metrics.NewHandler(r))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") && !strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Fatalf("Content-Type = %q, want text/plain or openmetrics", ct)
	}

	body := readAll(t, resp.Body)
	if !strings.Contains(body, "sablier_session_requests_total") {
		t.Errorf("body missing sablier_session_requests_total, got:\n%s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("body missing go_goroutines, got:\n%s", body)
	}
}

func readAll(t *testing.T, body interface{ Read(p []byte) (int, error) }) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/metrics/...`
Expected: FAIL — `metrics.NewHandler undefined`.

- [ ] **Step 3: Implement the handler**

Create `pkg/metrics/handler.go`:

```go
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler returns an http.Handler that serves the Prometheus exposition
// format for the given recorder's registry.
func NewHandler(r *PromRecorder) http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{
		Registry: r.registry,
	})
}

// Ensure registry satisfies the Gatherer interface used by promhttp.
var _ prometheus.Gatherer = (*prometheus.Registry)(nil)
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/metrics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/handler.go pkg/metrics/handler_test.go
git commit -m "feat(metrics): HTTP handler factory for /metrics"
```

---

## Task 11: Add `WithMetrics` and `Groups()` to `Sablier`

**Files:**
- Modify: `pkg/sablier/sablier.go`

- [ ] **Step 1: Edit `pkg/sablier/sablier.go`**

Replace the file with:

```go
package sablier

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sablierapp/sablier/pkg/metrics"
)

type Sablier struct {
	provider Provider
	sessions Store

	groupsMu sync.RWMutex
	groups   map[string][]string

	pendingMu     sync.Mutex
	pendingStarts map[string]*pendingStart

	// BlockingRefreshFrequency is the frequency at which the instances are checked
	// against the provider. Defaults to 5 seconds.
	BlockingRefreshFrequency time.Duration

	// InstanceStartTimeout is the maximum time allowed for an async InstanceStart
	// call before it is cancelled. Defaults to 5 minutes.
	InstanceStartTimeout time.Duration

	metrics metrics.Recorder

	l *slog.Logger
}

func New(logger *slog.Logger, store Store, provider Provider) *Sablier {
	return &Sablier{
		provider:                 provider,
		sessions:                 store,
		groupsMu:                 sync.RWMutex{},
		groups:                   map[string][]string{},
		pendingStarts:            map[string]*pendingStart{},
		l:                        logger,
		metrics:                  metrics.Noop{},
		BlockingRefreshFrequency: 5 * time.Second,
		InstanceStartTimeout:     5 * time.Minute,
	}
}

// WithMetrics installs a Recorder. Defaults to metrics.Noop until called.
func (s *Sablier) WithMetrics(r metrics.Recorder) {
	if r == nil {
		r = metrics.Noop{}
	}
	s.metrics = r
}

// Groups returns a defensive copy of the current group→instances map. Safe for
// concurrent use; intended for the metrics GroupLockCollector.
func (s *Sablier) Groups() map[string][]string {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	out := make(map[string][]string, len(s.groups))
	for k, v := range s.groups {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func (s *Sablier) SetGroups(groups map[string][]string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	if groups == nil {
		groups = map[string][]string{}
	}
	if diff := cmp.Diff(s.groups, groups); diff != "" {
		// TODO: Change this log for a friendly logging, groups rarely change, so we can put some effort on displaying what changed
		s.l.Info("set groups", slog.Any("old", s.groups), slog.Any("new", groups), slog.Any("diff", diff))
		s.groups = groups
	}
}

func (s *Sablier) RemoveInstance(ctx context.Context, name string) error {
	return s.sessions.Delete(ctx, name)
}
```

- [ ] **Step 2: Verify the package builds**

Run: `go build ./pkg/sablier/...`
Expected: clean build.

- [ ] **Step 3: Verify the existing tests still pass**

Run: `go test ./pkg/sablier/...`
Expected: PASS — no behavior changes for existing callers (Noop is the default).

- [ ] **Step 4: Commit**

```bash
git add pkg/sablier/sablier.go
git commit -m "feat(sablier): WithMetrics setter and Groups accessor"
```

---

## Task 12: Wire metrics into `requestStart` and `InstanceRequest`

**Files:**
- Modify: `pkg/sablier/instance_request.go`

- [ ] **Step 1: Read the current file to keep edits surgical**

Re-read `pkg/sablier/instance_request.go` to confirm the `requestStart` and `InstanceRequest` structure has not drifted from what's described below.

- [ ] **Step 2: Edit `requestStart` to record begin/active and time the start call**

Replace the body of `requestStart` (the function definition is unchanged) with:

```go
func (s *Sablier) requestStart(ctx context.Context, name string) (InstanceInfo, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	if ps, exists := s.pendingStarts[name]; exists {
		select {
		case <-ps.done:
			// Goroutine completed
			if ps.err != nil {
				err := ps.err
				delete(s.pendingStarts, name)
				return InstanceInfo{}, fmt.Errorf("instance start failed: %w", err)
			}
			// Succeeded previously but instance is no longer in store — start a new one
			delete(s.pendingStarts, name)
		default:
			// Still running — don't start another goroutine
			s.l.DebugContext(ctx, "instance start already in progress", slog.String("instance", name))
			return NotReadyInstanceState(name, 0, 1), nil
		}
	}

	ps := &pendingStart{done: make(chan struct{})}
	s.pendingStarts[name] = ps

	// Begin metrics tracking BEFORE dispatching the goroutine.
	// Idempotent — if a previous Begin was already recorded, it is preserved.
	s.metrics.RecordReadyWaitBegin(name)
	s.metrics.RecordActiveInstance(name)

	// Detach from the request context to avoid retaining HTTP request values,
	// but use a bounded timeout to prevent goroutine leaks.
	startCtx, cancel := context.WithTimeout(context.Background(), s.InstanceStartTimeout)

	go func() {
		defer cancel()
		defer close(ps.done)
		startedAt := time.Now()
		if err := s.provider.InstanceStart(startCtx, name); err != nil {
			ps.err = err
			s.metrics.RecordInstanceStartFailure(name)
			s.l.Error("async instance start failed", slog.String("instance", name), slog.Any("error", err))
		} else {
			s.metrics.RecordInstanceStartEnd(name, time.Since(startedAt))
			s.l.InfoContext(ctx, "instance is ready", slog.String("instance", name))
			// Success — clean up immediately so the entry doesn't linger
			s.pendingMu.Lock()
			// Only delete if ps is still the current entry (not replaced by a retry)
			if current, ok := s.pendingStarts[name]; ok && current == ps {
				delete(s.pendingStarts, name)
			}
			s.pendingMu.Unlock()
		}
	}()

	return NotReadyInstanceState(name, 0, 1), nil
}
```

- [ ] **Step 3: Edit `InstanceRequest` to record ready-wait end on the ready transition**

In `InstanceRequest`, find the block that calls `s.provider.InstanceInspect(ctx, name)` after the `pending` check and adjust it so a ready transition fires `RecordReadyWaitEnd`:

```go
		} else {
			s.l.DebugContext(ctx, "request to check instance status received", slog.String("instance", name), slog.String("current_status", string(state.Status)))
			state, err = s.provider.InstanceInspect(ctx, name)
			if err != nil {
				return InstanceInfo{}, err
			}
			if state.Status == InstanceStatusReady {
				s.metrics.RecordReadyWaitEnd(name)
			}
			s.l.DebugContext(ctx, "request to check instance status completed", slog.String("instance", name), slog.String("new_status", string(state.Status)))
		}
```

- [ ] **Step 4: Verify the existing tests still pass with Noop**

Run: `go test ./pkg/sablier/...`
Expected: PASS — Noop swallows all calls.

- [ ] **Step 5: Commit**

```bash
git add pkg/sablier/instance_request.go
git commit -m "feat(sablier): emit metrics for instance starts and ready transitions"
```

---

## Task 13: Add `fakeRecorder` and tests asserting metric calls in `requestStart`

**Files:**
- Modify: `pkg/sablier/instance_request_test.go`
- Modify: `pkg/sablier/sablier_test.go`

- [ ] **Step 1: Add a fakeRecorder helper and a setup variant in `sablier_test.go`**

Append to `pkg/sablier/sablier_test.go`:

```go
import (
	"sync"
	"time"

	"github.com/sablierapp/sablier/pkg/metrics"
)

type fakeRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeRecorder) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}

func (f *fakeRecorder) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeRecorder) RecordSessionRequest(strategy, target string) {
	f.record("session:" + strategy + "/" + target)
}
func (f *fakeRecorder) RecordInstanceStartEnd(instance string, _ time.Duration) {
	f.record("start_end:" + instance)
}
func (f *fakeRecorder) RecordInstanceStartFailure(instance string) {
	f.record("start_fail:" + instance)
}
func (f *fakeRecorder) RecordReadyWaitBegin(instance string)  { f.record("ready_begin:" + instance) }
func (f *fakeRecorder) RecordReadyWaitEnd(instance string)    { f.record("ready_end:" + instance) }
func (f *fakeRecorder) RecordActiveInstance(instance string)  { f.record("active+:" + instance) }
func (f *fakeRecorder) RecordInactiveInstance(instance string) {
	f.record("active-:" + instance)
}
func (f *fakeRecorder) RecordInstanceStop(instance, reason string) {
	f.record("stop:" + instance + "/" + reason)
}

// setupSablierWithMetrics is like setupSablier but installs a fakeRecorder.
func setupSablierWithMetrics(t *testing.T) (*sablier.Sablier, *storetest.MockStore, *providertest.MockProvider, *fakeRecorder) {
	t.Helper()
	m, s, p := setupSablier(t)
	r := &fakeRecorder{}
	var _ metrics.Recorder = r // compile-time interface check
	m.WithMetrics(r)
	return m, s, p, r
}
```

Make sure the imports at the top of `sablier_test.go` include `time`, `sync`, and `github.com/sablierapp/sablier/pkg/metrics` (in addition to what's already there). The existing imports remain untouched.

- [ ] **Step 2: Add a test for the success path in `instance_request_test.go`**

Append to `pkg/sablier/instance_request_test.go`:

```go
func TestInstanceRequest_NewInstance_RecordsStartMetrics_Success(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	startDone := make(chan struct{})

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ interface{}, _ string) error {
		close(startDone)
		return nil
	})

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	select {
	case <-startDone:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutine never completed")
	}
	// Settle for the goroutine to record the end metric.
	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		for _, c := range rec.snapshot() {
			if c == "start_end:nginx" {
				return true
			}
		}
		return false
	}), "expected start_end metric")

	calls := rec.snapshot()
	assertContains(t, calls, "ready_begin:nginx")
	assertContains(t, calls, "active+:nginx")
}

func TestInstanceRequest_NewInstance_RecordsStartFailure(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, sablier.NotReadyInstanceState("nginx", 0, 1), time.Minute).Return(nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("boom"))

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err) // first call returns not-ready, error surfaces on next

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		for _, c := range rec.snapshot() {
			if c == "start_fail:nginx" {
				return true
			}
		}
		return false
	}), "expected start_fail metric")

	calls := rec.snapshot()
	for _, c := range calls {
		if c == "start_end:nginx" {
			t.Errorf("did not expect start_end on failure, got: %v", calls)
		}
	}
}

func TestInstanceRequest_ReadyTransition_RecordsReadyEnd(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	notReady := sablier.NotReadyInstanceState("nginx", 0, 1)
	ready := sablier.ReadyInstanceState("nginx", 1)

	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(ready, nil)
	sessions.EXPECT().Put(ctx, ready, time.Minute).Return(nil)

	// Pre-seed the ready-wait state by simulating a previous Begin.
	rec.RecordReadyWaitBegin("nginx")

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	calls := rec.snapshot()
	assertContains(t, calls, "ready_end:nginx")
}

func assertContains(t *testing.T, calls []string, want string) {
	t.Helper()
	for _, c := range calls {
		if c == want {
			return
		}
	}
	t.Errorf("expected %q in calls, got: %v", want, calls)
}
```

- [ ] **Step 3: Run the new tests**

Run: `go test ./pkg/sablier/... -run 'RecordsStartMetrics|RecordsStartFailure|ReadyTransition_RecordsReadyEnd' -v`
Expected: PASS.

- [ ] **Step 4: Run the whole sablier package to confirm no regressions**

Run: `go test ./pkg/sablier/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/sablier/instance_request_test.go pkg/sablier/sablier_test.go
git commit -m "test(sablier): assert metrics calls in instance request flow"
```

---

## Task 14: Wire metrics into `OnInstanceExpired`

**Files:**
- Modify: `pkg/sablier/instance_expired.go`
- Modify: `pkg/sabliercmd/start.go` (call site update)

- [ ] **Step 1: Add a recorder parameter to `OnInstanceExpired`**

Replace `pkg/sablier/instance_expired.go` with:

```go
package sablier

import (
	"context"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/metrics"
)

// OnInstanceExpired returns a store-expiration callback that stops the
// instance via the provider and records the corresponding metrics.
//
// recorder may be metrics.Noop{} when metrics are disabled — call sites
// must always pass a non-nil recorder.
func OnInstanceExpired(ctx context.Context, provider Provider, recorder metrics.Recorder, logger *slog.Logger) func(string) {
	return func(_key string) {
		go func(key string) {
			logger.InfoContext(ctx, "instance expired", slog.String("instance", key))
			err := provider.InstanceStop(ctx, key)
			if err != nil {
				logger.ErrorContext(ctx, "instance expired could not be stopped from provider", slog.String("instance", key), slog.Any("error", err))
			}
			recorder.RecordInstanceStop(key, "expired")
			recorder.RecordInactiveInstance(key)
		}(_key)
	}
}
```

- [ ] **Step 2: Update the call site in `pkg/sabliercmd/start.go`**

In `Start`, change the line:

```go
	err = store.OnExpire(ctx, sablier.OnInstanceExpired(ctx, provider, logger))
```

to:

```go
	rec := buildRecorder(conf.Server.Metrics.Enabled)
	err = store.OnExpire(ctx, sablier.OnInstanceExpired(ctx, provider, rec, logger))
```

`buildRecorder` will be defined in Task 18 — for now leave it referenced as a forward declaration. To keep this task building on its own, add a minimal helper at the bottom of `start.go`:

```go
func buildRecorder(enabled bool) metrics.Recorder {
	if enabled {
		return metrics.NewPromRecorder()
	}
	return metrics.Noop{}
}
```

and add the import: `"github.com/sablierapp/sablier/pkg/metrics"`.

Pass `rec` through to `s.WithMetrics(rec)` immediately after `s := sablier.New(...)`. Task 18 will refine wiring (collector registration, ServeStrategy field) — for now we just need the package to compile.

- [ ] **Step 3: Build and run all tests**

Run: `go build ./... && go test ./pkg/sablier/... ./pkg/sabliercmd/...`
Expected: PASS. The `pkg/sabliercmd` tests use `mockStartCommand` which intercepts `Start` before it's called, so the new recorder construction is harmless there.

- [ ] **Step 4: Commit**

```bash
git add pkg/sablier/instance_expired.go pkg/sabliercmd/start.go
git commit -m "feat(sablier): record instance stops on session expiry"
```

---

## Task 15: Wire stop counter into `StopAllUnregisteredInstances`

**Files:**
- Modify: `pkg/sablier/autostop.go`

- [ ] **Step 1: Edit `stopFunc` to record the metric on success**

In `pkg/sablier/autostop.go`, replace `stopFunc` with:

```go
func (s *Sablier) stopFunc(ctx context.Context, name string) func() error {
	return func() error {
		err := s.provider.InstanceStop(ctx, name)
		if err != nil {
			s.l.ErrorContext(ctx, "failed to stop instance", slog.String("instance", name), slog.Any("error", err))
			return err
		}
		s.metrics.RecordInstanceStop(name, "unregistered")
		s.l.InfoContext(ctx, "stopped unregistered instance", slog.String("instance", name), slog.String("reason", "instance is enabled but not started by Sablier"))
		return nil
	}
}
```

- [ ] **Step 2: Build and run tests**

Run: `go build ./... && go test ./pkg/sablier/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add pkg/sablier/autostop.go
git commit -m "feat(sablier): record stop counter for unregistered instances"
```

---

## Task 16: Add `Metrics` to `ServeStrategy` and emit session counters in API handlers

**Files:**
- Modify: `internal/api/api.go`
- Modify: `internal/api/start_dynamic.go`
- Modify: `internal/api/start_blocking.go`

- [ ] **Step 1: Add the field to `ServeStrategy`**

Replace `internal/api/api.go` with:

```go
package api

import (
	"context"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package apitest -source=api.go -destination=apitest/mocks_sablier.go *

type Sablier interface {
	RequestSession(ctx context.Context, names []string, duration time.Duration) (*sablier.SessionState, error)
	RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (*sablier.SessionState, error)
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*sablier.SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*sablier.SessionState, error)
}

type ServeStrategy struct {
	Theme *theme.Themes

	Sablier        Sablier
	Metrics        metrics.Recorder
	StrategyConfig config.Strategy
	SessionsConfig config.Sessions
}
```

- [ ] **Step 2: Increment counter in `start_dynamic.go`**

In `internal/api/start_dynamic.go`, after the validation block (where it has confirmed exactly one of names/group is set), add immediately before the `if len(request.Names) > 0` switch:

```go
		target := "names"
		if request.Group != "" {
			target = "group"
		}
		if s.Metrics != nil {
			s.Metrics.RecordSessionRequest("dynamic", target)
		}
```

- [ ] **Step 3: Same change in `start_blocking.go`**

Same pattern, but with `"blocking"`:

```go
		target := "names"
		if request.Group != "" {
			target = "group"
		}
		if s.Metrics != nil {
			s.Metrics.RecordSessionRequest("blocking", target)
		}
```

- [ ] **Step 4: Build and run all tests**

Run: `go build ./... && go test ./internal/...`
Expected: PASS. Existing API tests don't set `s.Metrics`, but the nil check above keeps them working. (The proper fix is below — set it to `Noop` in `setupServeStrategy` test helpers if any exist; check `internal/api/apitest/` and `internal/api/start_*_test.go`.)

- [ ] **Step 5: If apitest helpers construct `ServeStrategy`, default `Metrics` to `Noop`**

Run: `grep -rn "ServeStrategy{" internal/api/`
For each match, ensure `Metrics: metrics.Noop{}` is set (import `github.com/sablierapp/sablier/pkg/metrics`). This avoids the nil check above being relied on long-term.

After updating helpers, remove the `if s.Metrics != nil` guards added in steps 2–3 (call directly: `s.Metrics.RecordSessionRequest(...)`).

- [ ] **Step 6: Run all internal tests again**

Run: `go test ./internal/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/
git commit -m "feat(api): record session request counter from strategy handlers"
```

---

## Task 17: Register `/metrics` route conditionally + integration test

**Files:**
- Modify: `internal/server/routes.go`
- Modify: `internal/server/server.go` (signature change to plumb the recorder)
- Create: `internal/server/metrics_test.go`

- [ ] **Step 1: Plumb the recorder through `setupRouter` / `Start`**

The current `Start(ctx, logger, serverConf, s)` already receives the `*api.ServeStrategy`, which (after Task 16) has a `Metrics` field. No signature change is needed — `routes.go` reads `s.Metrics`.

- [ ] **Step 2: Edit `routes.go` to register `/metrics` conditionally**

Replace `internal/server/routes.go` with:

```go
package server

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
)

func registerRoutes(ctx context.Context, router *gin.Engine, serverConf config.Server, s *api.ServeStrategy) {
	router.RedirectTrailingSlash = true

	base := router.Group(serverConf.BasePath)

	api.Healthcheck(base, ctx)

	// Register /metrics only when a real PromRecorder is in use.
	if rec, ok := s.Metrics.(*metrics.PromRecorder); ok {
		base.GET("/metrics", gin.WrapH(metrics.NewHandler(rec)))
	}

	APIv1 := base.Group("/api")
	api.StartDynamic(APIv1, s)
	api.StartBlocking(APIv1, s)
	api.ListThemes(APIv1, s)
}
```

- [ ] **Step 3: Write the integration test**

Create `internal/server/metrics_test.go`:

```go
package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestMetricsEndpoint_EnabledServesPrometheusExposition(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := metrics.NewPromRecorder()
	rec.RecordSessionRequest("dynamic", "names")

	strategy := &api.ServeStrategy{
		Metrics: rec,
	}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "sablier_session_requests_total") {
		t.Errorf("body missing sablier_session_requests_total; got:\n%s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("body missing go_goroutines; got:\n%s", body)
	}
}

func TestMetricsEndpoint_DisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	strategy := &api.ServeStrategy{
		Metrics: metrics.Noop{},
	}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestMetricsEndpoint_RespectsBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := metrics.NewPromRecorder()
	strategy := &api.ServeStrategy{Metrics: rec}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/sablier"}, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sablier/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
```

- [ ] **Step 4: Run the integration tests**

Run: `go test ./internal/server/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/routes.go internal/server/metrics_test.go
git commit -m "feat(server): expose /metrics endpoint when enabled"
```

---

## Task 18: Wire everything in `start.go` (collector registration, ServeStrategy)

**Files:**
- Modify: `pkg/sabliercmd/start.go`

- [ ] **Step 1: Replace the relevant block of `Start`**

Find the section in `pkg/sabliercmd/start.go` that constructs the recorder (added in Task 14), the `Sablier`, and the `ServeStrategy`. Replace that section to:

```go
	rec := buildRecorder(conf.Server.Metrics.Enabled)

	store := inmemory.NewInMemory()
	err = store.OnExpire(ctx, sablier.OnInstanceExpired(ctx, provider, rec, logger))
	if err != nil {
		return err
	}

	s := sablier.New(logger, store, provider)
	s.WithMetrics(rec)
	s.BlockingRefreshFrequency = conf.Strategy.Blocking.DefaultRefreshFrequency

	// Register the GroupLockCollector on the same registry so the gauges show
	// up alongside everything else when /metrics is scraped.
	if pr, ok := rec.(*metrics.PromRecorder); ok {
		pr.Registry().MustRegister(metrics.NewGroupLockCollector(s, pr))
	}

	groups, err := provider.InstanceGroups(ctx)
	if err != nil {
		logger.WarnContext(ctx, "initial group scan failed", slog.Any("reason", err))
	} else {
		s.SetGroups(groups)
	}

	go s.GroupWatch(ctx)
	instanceStopped := make(chan string)
	go provider.NotifyInstanceStopped(ctx, instanceStopped)
	go func() {
		for stopped := range instanceStopped {
			err := s.RemoveInstance(ctx, stopped)
			if err != nil {
				logger.Warn("could not remove instance", slog.Any("error", err))
			}
		}
	}()

	if conf.Provider.AutoStopOnStartup {
		err := s.StopAllUnregisteredInstances(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "unable to stop unregistered instances", slog.Any("reason", err))
		}
	}

	t, err := setupTheme(ctx, conf, logger)
	if err != nil {
		return fmt.Errorf("cannot setup theme: %w", err)
	}

	strategy := &api.ServeStrategy{
		Theme:          t,
		Sablier:        s,
		Metrics:        rec,
		StrategyConfig: conf.Strategy,
		SessionsConfig: conf.Sessions,
	}

	go server.Start(ctx, logger, conf.Server, strategy)
```

- [ ] **Step 2: Build the binary and run all tests**

Run: `go build ./... && go test ./...`
Expected: PASS across the board.

- [ ] **Step 3: Smoke-test by running with metrics enabled**

Run, in a separate terminal:

```bash
SABLIER_PROVIDER_NAME=docker SABLIER_SERVER_METRICS_ENABLED=true \
  go run ./cmd/sablier start --provider.name docker --server.metrics.enabled
```

Then in another terminal:

```bash
curl -s http://localhost:10000/metrics | head -40
```

Expected: Prometheus exposition output containing `sablier_session_requests_total` (zero value, no series until first request), `sablier_group_locked` (one series per group if any are configured), `go_goroutines`, `process_*`. Stop the server with Ctrl+C.

If the user's environment can't run a Docker provider, skip Step 3 and rely on the unit + integration tests.

- [ ] **Step 4: Commit**

```bash
git add pkg/sabliercmd/start.go
git commit -m "feat(cmd): wire Prometheus recorder, collector, and ServeStrategy"
```

---

## Task 19: Documentation

**Files:**
- Modify: `sablier.sample.yaml`
- Modify: `docs/configuration.md`

- [ ] **Step 1: Add the option to the sample YAML**

In `sablier.sample.yaml`, change the `server` block to:

```yaml
server:
  port: 10000
  base-path: /
  metrics:
    enabled: true
```

(Setting it to `true` in the sample so users can see the option exists; the default is still `false`.)

- [ ] **Step 2: Document in `docs/configuration.md`**

Find the `server:` section in `docs/configuration.md` and add documentation for `server.metrics.enabled`. Include:

- Default: `false`.
- Endpoint: `<base-path>/metrics` when enabled.
- Format: Prometheus text exposition.
- Security note: The endpoint exposes process internals, group/instance names, and counters. It is intended for trusted observability stacks. Restrict at the reverse proxy when Sablier is fronted on an untrusted network.
- Metrics table summarizing the seven Sablier metrics + the inclusion of standard Go and process collectors. (Reuse the table from the proposal spec at `docs/proposals/2026-05-05-prometheus-metrics.md`.)

Add the matching CLI flag table entry:

```
--server.metrics.enabled                                Enable the Prometheus /metrics endpoint (default false)
```

- [ ] **Step 3: Commit**

```bash
git add sablier.sample.yaml docs/configuration.md
git commit -m "docs(metrics): document server.metrics.enabled and /metrics endpoint"
```

---

## Task 20: Final verification

**Files:** none

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Confirm `go.mod` has the direct dep**

Run: `grep prometheus/client_golang go.mod`
Expected: a line under the top-level `require ( ... )` block (i.e., not commented as `// indirect`).

- [ ] **Step 4: Check the diff is reasonable**

Run: `git diff main --stat`
Expected: roughly 15–20 files changed: new `pkg/metrics/` package, edits to sablier core, api handlers, server, cmd, config, docs.

- [ ] **Step 5: Confirm the branch is clean**

Run: `git status`
Expected: `nothing to commit, working tree clean` (modulo `.claude/` and `.env.gitlab` which are local/untracked).

The implementation is ready for upstream review.

---

## Self-review summary

- **Spec coverage:** Every metric in the catalog has a Record method, every Record method is exercised by a test, and each call site listed in the spec has a corresponding task. Config (`server.metrics.enabled`), endpoint registration (`base-path/metrics` when enabled), security note, sample YAML, and `go.mod` promotion all addressed.
- **Type consistency:** `Recorder` interface defined once in Task 3 and used identically by call sites in Tasks 12–16 and the test fake in Task 13. `RecordInstanceStartEnd(instance string, dur time.Duration)` (no error param) is consistent throughout.
- **Risks worth flagging during implementation:**
  - `pkg/sabliercmd/testdata/config_default.json` snapshot (Task 2 Step 3) — exact JSON shape depends on how viper marshals the nested struct; if the snapshot diff is large, hand-check it before committing.
  - Test files referenced (e.g. `internal/api/apitest`) may construct `ServeStrategy` literals; Task 16 Step 5 exists specifically to find those.
  - The integration test in Task 17 runs against `setupRouter` directly and bypasses `server.Start`'s lifecycle. That's intentional — the lifecycle is irrelevant to route registration.
