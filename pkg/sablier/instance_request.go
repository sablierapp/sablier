package sablier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/store"
)

type pendingStart struct {
	done    chan struct{}
	err     error
	info    InstanceInfo      // set at creation time; used to return consistent data while starting
	spanCtx trace.SpanContext // OTel context of the triggering request; used to propagate the trace into the goroutine
}

// consumePendingError checks whether a pending start exists for the given
// instance. It returns (pending, error):
//   - pending=true, err=nil  -> start still in progress, caller should skip inspect
//   - pending=false, err!=nil -> start completed with error, entry cleared for retry
//   - pending=false, err=nil  -> no pending entry or already cleaned up
func (s *Sablier) consumePendingError(name string) (bool, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	ps, exists := s.pendingStarts[name]
	if !exists {
		return false, nil
	}

	select {
	case <-ps.done:
		// Goroutine finished — clean up regardless of outcome
		delete(s.pendingStarts, name)
		if ps.err != nil {
			return false, fmt.Errorf("instance start failed: %w", ps.err)
		}
		return false, nil
	default:
		// Still running
		return true, nil
	}
}

func (s *Sablier) requestStart(ctx context.Context, name string, rejectUnlabeled bool) (InstanceInfo, error) {
	// First critical section: check whether a start is already in progress.
	// We release the lock before doing the remote inspect to avoid holding a
	// mutex across a potentially slow network call.
	s.pendingMu.Lock()
	if ps, exists := s.pendingStarts[name]; exists {
		select {
		case <-ps.done:
			// Goroutine completed
			if ps.err != nil {
				err := ps.err
				delete(s.pendingStarts, name)
				s.pendingMu.Unlock()
				return InstanceInfo{}, fmt.Errorf("instance start failed: %w", err)
			}
			// Succeeded previously but instance is no longer in store — fall through to restart
			delete(s.pendingStarts, name)
		default:
			// Still running — return the cached InstanceInfo to preserve provider fields
			s.l.DebugContext(ctx, "instance start already in progress", slog.String("instance", name))
			// Record that this request joined an already-running start goroutine so the
			// two traces can be correlated in the backend.
			trace.SpanFromContext(ctx).AddEvent("sablier.instance.join_pending_start",
				trace.WithAttributes(
					attribute.String("instance", name),
					attribute.String("pending_trace_id", ps.spanCtx.TraceID().String()),
					attribute.String("pending_span_id", ps.spanCtx.SpanID().String()),
				),
			)
			info := ps.info
			s.pendingMu.Unlock()
			return info, nil
		}
	}
	s.pendingMu.Unlock()

	// Inspect outside the lock: this may be a slow remote network call.
	// If inspect fails (e.g. first boot), fall back to a minimal struct so
	// the start still proceeds.
	inspectCtx, inspectSpan := s.tracer.Start(ctx, "sablier.instance.inspect",
		trace.WithAttributes(attribute.String("instance", name)))
	info, err := s.provider.InstanceInspect(inspectCtx, name)
	inspectSpan.RecordError(err)
	if err != nil {
		inspectSpan.SetStatus(codes.Error, err.Error())
	}
	inspectSpan.End()
	if err != nil {
		if rejectUnlabeled {
			return InstanceInfo{}, err
		}
		s.l.DebugContext(ctx, "pre-start inspect failed, using bare info", slog.String("instance", name), slog.Any("error", err))
		info = InstanceInfo{Name: name, CurrentReplicas: 0, DesiredReplicas: 1}
	}
	if rejectUnlabeled && info.Enabled != "true" {
		return InstanceInfo{}, ErrInstanceNotManaged{Name: name}
	}
	info.Status = InstanceStatusStarting

	// Second critical section: register the pending entry. Re-check in case
	// another goroutine raced past the first unlock and registered first.
	s.pendingMu.Lock()
	if existing, exists := s.pendingStarts[name]; exists {
		select {
		case <-existing.done:
			// The racing goroutine already finished; proceed with our own entry.
			delete(s.pendingStarts, name)
		default:
			// A concurrent goroutine won the race; return its cached info.
			s.l.DebugContext(ctx, "instance start already in progress (post-inspect race)", slog.String("instance", name))
			trace.SpanFromContext(ctx).AddEvent("sablier.instance.join_pending_start",
				trace.WithAttributes(
					attribute.String("instance", name),
					attribute.String("pending_trace_id", existing.spanCtx.TraceID().String()),
					attribute.String("pending_span_id", existing.spanCtx.SpanID().String()),
				),
			)
			existingInfo := existing.info
			s.pendingMu.Unlock()
			return existingInfo, nil
		}
	}
	// Capture the OTel span context before releasing the lock so the goroutine
	// can be parented to the triggering request's trace.
	spanCtx := trace.SpanContextFromContext(ctx)
	ps := &pendingStart{done: make(chan struct{}), info: info, spanCtx: spanCtx}
	s.pendingStarts[name] = ps
	s.pendingMu.Unlock()

	// Begin metrics tracking BEFORE dispatching the goroutine.
	// Idempotent — if a previous Begin was already recorded, it is preserved.
	s.metrics.RecordReadyWaitBegin(name)
	s.metrics.RecordActiveInstance(name)

	// Build a background context that carries the OTel span context from the
	// triggering HTTP request so the async InstanceStart call appears as a child
	// of that request's trace, without being bound by its cancellation or deadline.
	startCtx, cancel := context.WithTimeout(
		trace.ContextWithSpanContext(context.Background(), spanCtx),
		s.InstanceStartTimeout,
	)

	go func() {
		defer cancel()
		defer close(ps.done)
		startedAt := time.Now()
		startCtx, startSpan := s.tracer.Start(startCtx, "sablier.instance.start",
			trace.WithAttributes(attribute.String("instance", name)))
		if err := s.provider.InstanceStart(startCtx, name); err != nil {
			startSpan.RecordError(err)
			startSpan.SetStatus(codes.Error, err.Error())
			startSpan.End()
			ps.err = err
			s.metrics.RecordInstanceStartFailure(name)
			s.l.ErrorContext(startCtx, "async instance start failed", slog.String("instance", name), slog.Any("error", err))
		} else {
			startSpan.End()
			s.metrics.RecordInstanceStartEnd(name, time.Since(startedAt))
			s.l.InfoContext(startCtx, "instance is ready", slog.String("instance", name))
			// Success — clean up immediately so the entry doesn't linger
			s.pendingMu.Lock()
			// Only delete if ps is still the current entry (not replaced by a retry)
			if current, ok := s.pendingStarts[name]; ok && current == ps {
				delete(s.pendingStarts, name)
			}
			s.pendingMu.Unlock()
		}
	}()

	return info, nil
}

func (s *Sablier) InstanceRequest(ctx context.Context, name string, duration time.Duration) (InstanceInfo, error) {
	return s.instanceRequest(ctx, name, duration, false)
}

func (s *Sablier) instanceRequest(ctx context.Context, name string, duration time.Duration, rejectUnlabeled bool) (InstanceInfo, error) {
	if name == "" {
		return InstanceInfo{}, errors.New("instance name cannot be empty")
	}

	state, err := s.sessions.Get(ctx, name)
	if errors.Is(err, store.ErrKeyNotFound) {
		s.l.DebugContext(ctx, "request to start instance received", slog.String("instance", name))

		state, err = s.requestStart(ctx, name, rejectUnlabeled)
		if err != nil {
			return InstanceInfo{}, err
		}

		s.l.InfoContext(ctx, "request to start instance dispatched", slog.String("instance", name), slog.String("status", string(state.Status)), slog.Duration("expiration", duration))
	} else if err != nil {
		s.l.ErrorContext(ctx, "request to start instance failed", slog.String("instance", name), slog.Any("error", err))
		return InstanceInfo{}, fmt.Errorf("cannot retrieve instance from store: %w", err)
	} else if state.Status != InstanceStatusReady {
		// Check for a completed (possibly failed) async start before inspecting
		pending, pendingErr := s.consumePendingError(name)
		if pendingErr != nil {
			return InstanceInfo{}, pendingErr
		}

		if pending {
			// Start is still in progress — no point inspecting, return current state
			s.l.DebugContext(ctx, "instance start still in progress, skipping inspect", slog.String("instance", name))
		} else {
			s.l.DebugContext(ctx, "request to check instance status received", slog.String("instance", name), slog.String("current_status", string(state.Status)))
			inspectCtx, inspectSpan := s.tracer.Start(ctx, "sablier.instance.inspect",
				trace.WithAttributes(attribute.String("instance", name)))
			state, err = s.provider.InstanceInspect(inspectCtx, name)
			inspectSpan.RecordError(err)
			if err != nil {
				inspectSpan.SetStatus(codes.Error, err.Error())
			}
			inspectSpan.End()
			if err != nil {
				return InstanceInfo{}, err
			}
			if state.Status == InstanceStatusReady {
				s.metrics.RecordReadyWaitEnd(name)
				// First transition to ready — stamp the time so ReadyAfter can be enforced.
				now := time.Now()
				state.ReadyAt = &now
			}
			s.l.DebugContext(ctx, "request to check instance status completed", slog.String("instance", name), slog.String("new_status", string(state.Status)))
		}
	}

	effectiveDuration := duration
	if state.RunningHours != "" {
		remaining, inWindow, runningHoursErr := runningHoursRemaining(state.RunningHours, time.Now())
		if runningHoursErr != nil {
			s.l.WarnContext(ctx, "invalid running-hours value in state, ignoring", slog.String("instance", name), slog.String("value", state.RunningHours), slog.Any("error", runningHoursErr))
		} else if inWindow && remaining > effectiveDuration {
			effectiveDuration = remaining
			s.l.DebugContext(ctx, "running-hours window active, extending expiration", slog.String("instance", name), slog.Duration("expiration", effectiveDuration), slog.Duration("window_remaining", remaining))
		}
	}

	s.l.DebugContext(ctx, "set expiration for instance", slog.String("instance", name), slog.Duration("expiration", effectiveDuration))

	err = s.sessions.Put(ctx, state, effectiveDuration)
	if err != nil {
		s.l.ErrorContext(ctx, "could not put instance to store, will not expire", slog.Any("error", err), slog.String("instance", state.Name))
		return InstanceInfo{}, fmt.Errorf("could not put instance to store: %w", err)
	}
	return state, nil
}
