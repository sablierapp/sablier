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

func (r *PromRecorder) RecordSessionRequest(strategy, target string) {
	r.sessionRequests.WithLabelValues(strategy, target).Inc()
}

func (r *PromRecorder) RecordInstanceStartFailure(instance string) {
	r.instanceStartFailures.WithLabelValues(instance).Inc()
}

func (r *PromRecorder) RecordInstanceStop(instance, reason string) {
	r.instanceStops.WithLabelValues(instance, reason).Inc()
}

func (r *PromRecorder) RecordInstanceStartEnd(instance string, dur time.Duration) {
	r.instanceStartDuration.WithLabelValues(instance).Observe(dur.Seconds())
}

func (r *PromRecorder) RecordReadyWaitBegin(instance string) {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()
	if _, exists := r.readyWait[instance]; exists {
		return
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

// DiscardReadyWait clears any pending ready-wait timestamp for the instance
// without observing the histogram. Called when an instance is stopped or its
// session expires before becoming Ready, so the next start of the same
// instance does not observe a stale duration.
func (r *PromRecorder) DiscardReadyWait(instance string) {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()
	delete(r.readyWait, instance)
}

// ReadyWaitStartedAt returns the timestamp recorded for an instance's pending
// ready transition, if any. Test-only helper.
func (r *PromRecorder) ReadyWaitStartedAt(instance string) (time.Time, bool) {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()
	t, ok := r.readyWait[instance]
	return t, ok
}

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

// SnapshotActiveInstances returns a copy of the current active set.
func (r *PromRecorder) SnapshotActiveInstances() map[string]struct{} {
	r.activeMu.RLock()
	defer r.activeMu.RUnlock()
	out := make(map[string]struct{}, len(r.activeInstances))
	for k := range r.activeInstances {
		out[k] = struct{}{}
	}
	return out
}
