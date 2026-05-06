package metrics_test

import (
	"testing"

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
