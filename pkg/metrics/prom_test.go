package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
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
