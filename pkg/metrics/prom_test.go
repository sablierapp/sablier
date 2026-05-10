package metrics_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
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

	families, err := reg.(*prometheus.Registry).Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}

	if !names["go_goroutines"] {
		t.Errorf("expected go_goroutines metric to be registered, got: %v", keys(names))
	}
	if !names["process_start_time_seconds"] {
		t.Errorf("expected process_start_time_seconds metric to be registered, got: %v", keys(names))
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

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

// findMetric gathers the registry and returns the first metric matching
// (name, labels), or nil if not present.
func findMetric(t *testing.T, r *metrics.PromRecorder, name string, labels map[string]string) *io_prometheus_client.Metric {
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
			if labelsMatch(m.GetLabel(), labels) {
				return m
			}
		}
	}
	return nil
}

func mustCounter(t *testing.T, r *metrics.PromRecorder, name string, labels map[string]string, want float64) {
	t.Helper()
	m := findMetric(t, r, name, labels)
	if m == nil {
		t.Errorf("counter %s%v not found", name, labels)
		return
	}
	if got := m.GetCounter().GetValue(); got != want {
		t.Errorf("%s%v = %v, want %v", name, labels, got, want)
	}
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
	m := findMetric(t, r, name, labels)
	if m == nil {
		t.Errorf("histogram %s%v not found", name, labels)
		return
	}
	if got := m.GetHistogram().GetSampleCount(); got != want {
		t.Errorf("%s%v sample count = %d, want %d", name, labels, got, want)
	}
}

// assertNoHistogramSamples asserts that no samples have been observed on the
// named histogram for the given labels.
func assertNoHistogramSamples(t *testing.T, r *metrics.PromRecorder, name string, labels map[string]string) {
	t.Helper()
	if m := findMetric(t, r, name, labels); m != nil {
		if got := m.GetHistogram().GetSampleCount(); got != 0 {
			t.Errorf("expected no samples on %s%v but found %d", name, labels, got)
		}
	}
}

func TestPromRecorder_ReadyWait_BeginThenEnd(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordReadyWaitBegin("nginx")
	r.RecordReadyWaitEnd("nginx")

	mustHistogramCount(t, r, "sablier_instance_ready_duration_seconds", map[string]string{"instance": "nginx"}, 1)
}

func TestPromRecorder_ReadyWait_EndWithoutBeginIsNoop(t *testing.T) {
	r := metrics.NewPromRecorder()
	r.RecordReadyWaitEnd("nginx")
	assertNoHistogramSamples(t, r, "sablier_instance_ready_duration_seconds", map[string]string{"instance": "nginx"})
}

func TestPromRecorder_ReadyWait_BeginIsIdempotent(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordReadyWaitBegin("nginx")
	first, ok := r.ReadyWaitStartedAt("nginx")
	if !ok {
		t.Fatal("expected timestamp")
	}
	time.Sleep(20 * time.Millisecond)
	r.RecordReadyWaitBegin("nginx")
	second, ok := r.ReadyWaitStartedAt("nginx")
	if !ok {
		t.Fatal("expected timestamp")
	}

	if !first.Equal(second) {
		t.Errorf("RecordReadyWaitBegin reset the timestamp: %v -> %v", first, second)
	}
}

func TestPromRecorder_DiscardReadyWaitDoesNotObserve(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordReadyWaitBegin("nginx")
	r.DiscardReadyWait("nginx")

	// Subsequent End must be a no-op (no entry exists).
	r.RecordReadyWaitEnd("nginx")

	assertNoHistogramSamples(t, r, "sablier_instance_ready_duration_seconds", map[string]string{"instance": "nginx"})

	// And the entry must be gone — calling Discard again is harmless.
	r.DiscardReadyWait("nginx")

	if _, ok := r.ReadyWaitStartedAt("nginx"); ok {
		t.Errorf("expected readyWait[nginx] to be cleared")
	}
}

func TestPromRecorder_ActiveInstances(t *testing.T) {
	r := metrics.NewPromRecorder()

	r.RecordActiveInstance("nginx")
	r.RecordActiveInstance("redis")
	r.RecordActiveInstance("nginx")

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
