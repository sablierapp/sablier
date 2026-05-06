package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// histogramBuckets covers container start times: 100ms to 5min.
var histogramBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300}

// PromRecorder is the Prometheus-backed Recorder.
type PromRecorder struct {
	registry *prometheus.Registry

	sessionRequests       *prometheus.CounterVec
	instanceStartFailures *prometheus.CounterVec
	instanceStops         *prometheus.CounterVec
	instanceStartDuration *prometheus.HistogramVec
	instanceReadyDuration *prometheus.HistogramVec

	activeMu        sync.RWMutex
	activeInstances map[string]struct{}

	readyMu   sync.Mutex
	readyWait map[string]time.Time
}

// NewPromRecorder constructs a PromRecorder with all metric vectors and the
// standard Go and process collectors registered on a fresh registry.
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

// Registry returns the underlying registry.
func (r *PromRecorder) Registry() prometheus.Registerer {
	return r.registry
}
