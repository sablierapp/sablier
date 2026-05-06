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

// Noop is the zero-overhead default recorder.
type Noop struct{}

func (Noop) RecordSessionRequest(string, string)          {}
func (Noop) RecordInstanceStartEnd(string, time.Duration) {}
func (Noop) RecordInstanceStartFailure(string)            {}
func (Noop) RecordReadyWaitBegin(string)                  {}
func (Noop) RecordReadyWaitEnd(string)                    {}
func (Noop) RecordActiveInstance(string)                  {}
func (Noop) RecordInactiveInstance(string)                {}
func (Noop) RecordInstanceStop(string, string)            {}
