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

// Delay before a request may start an instance again after its start failed,
// doubled after every consecutive failure up to startRetryMaxDelay.
const (
	startRetryBaseDelay = 2 * time.Second
	startRetryMaxDelay  = 2 * time.Minute
)

// failedStart records that the last start of an instance failed, so a later
// request for its still-active session starts it again instead of only
// inspecting it. Entries are dropped when a start succeeds and whenever the
// instance is stopped on purpose (expiration, anti-affinity suppression, stop
// events), so only a failure Sablier observed itself ever triggers a retry.
type failedStart struct {
	failures int       // consecutive failed starts, drives the backoff
	retryAt  time.Time // earliest time a request may start the instance again
}

func startRetryDelay(failures int) time.Duration {
	delay := startRetryBaseDelay
	for i := 1; i < failures && delay < startRetryMaxDelay; i++ {
		delay *= 2
	}
	return min(delay, startRetryMaxDelay)
}

// forgetFailedStart drops any recorded start failure for name, so no request
// starts it again. Call it whenever the instance is stopped on purpose.
func (s *Sablier) forgetFailedStart(name string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.failedStarts, name)
}

// retryFailedStart starts name again when its last start failed, the backoff
// has elapsed and no start is in progress. The check and the registration of
// the new pending start happen under one lock, so concurrent requests dispatch
// at most one retry. session is the stored session state; it is reused as the
// pre-start info instead of inspecting the instance again.
func (s *Sablier) retryFailedStart(ctx context.Context, name string, session InstanceInfo) (InstanceInfo, bool) {
	s.pendingMu.Lock()
	fs, ok := s.failedStarts[name]
	if !ok || time.Now().Before(fs.retryAt) {
		s.pendingMu.Unlock()
		return InstanceInfo{}, false
	}
	// A start is in progress, or a finished one still holds an error no
	// request has reported yet (consumePendingError surfaces it first).
	if _, pending := s.pendingStarts[name]; pending {
		s.pendingMu.Unlock()
		return InstanceInfo{}, false
	}
	failures := fs.failures

	info := session
	info.Status = InstanceStatusStarting
	info.ReadyAt = nil
	ps := &pendingStart{done: make(chan struct{}), info: info, spanCtx: trace.SpanContextFromContext(ctx)}
	s.pendingStarts[name] = ps
	s.pendingMu.Unlock()

	s.l.WarnContext(ctx, "starting instance again after a failed start",
		slog.String("instance", name), slog.Int("failures", failures))
	s.dispatchStart(ctx, name, ps, true)
	return info, true
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

// startsDone returns a channel that is closed when every start in progress for
// names completes. It returns nil when no start is in progress.
func (s *Sablier) startsDone(names []string) <-chan struct{} {
	s.pendingMu.Lock()
	var pending []chan struct{}
	for _, name := range names {
		if ps, ok := s.pendingStarts[name]; ok {
			pending = append(pending, ps.done)
		}
	}
	s.pendingMu.Unlock()

	if len(pending) == 0 {
		return nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, ch := range pending {
			<-ch
		}
	}()
	return done
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
	if rejectUnlabeled && !info.IsEnabled() {
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

	s.dispatchStart(ctx, name, ps, false)
	return info, nil
}

// dispatchStart starts name and its dependencies in the background for the
// already-registered pending start ps. A failure is recorded in failedStarts
// so a later request retries it; a success clears it. For a retry, the
// instance is forced idle again if an anti-affinity hold became active while
// it was starting: a suppression that ran in the meantime may have stopped it
// before this start landed, and would not look at it again.
func (s *Sablier) dispatchStart(ctx context.Context, name string, ps *pendingStart, retry bool) {
	// Begin metrics tracking BEFORE dispatching the goroutine.
	// Idempotent — if a previous Begin was already recorded, it is preserved.
	s.metrics.RecordReadyWaitBegin(name)
	s.metrics.RecordActiveInstance(name)

	// Build a background context that carries the OTel span context from the
	// triggering HTTP request so the async InstanceStart call appears as a child
	// of that request's trace, without being bound by its cancellation or deadline.
	startCtx, cancel := context.WithTimeout(
		trace.ContextWithSpanContext(context.Background(), ps.spanCtx),
		s.InstanceStartTimeout,
	)

	go func() {
		defer cancel()
		defer close(ps.done)
		startedAt := time.Now()
		startCtx, startSpan := s.tracer.Start(startCtx, "sablier.instance.start",
			trace.WithAttributes(attribute.String("instance", name)))
		if err := s.startWithDependencies(startCtx, name); err != nil {
			startSpan.RecordError(err)
			startSpan.SetStatus(codes.Error, err.Error())
			startSpan.End()
			ps.err = err
			s.metrics.RecordInstanceStartFailure(name)
			s.l.ErrorContext(startCtx, "async instance start failed", slog.String("instance", name), slog.Any("error", err))
			s.pendingMu.Lock()
			fs, ok := s.failedStarts[name]
			if !ok {
				fs = &failedStart{}
				s.failedStarts[name] = fs
			}
			fs.failures++
			fs.retryAt = time.Now().Add(startRetryDelay(fs.failures))
			s.pendingMu.Unlock()
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
			delete(s.failedStarts, name)
			s.pendingMu.Unlock()
			if retry {
				s.undoRetryIfStoppedMeanwhile(startCtx, name)
			}
		}
	}()
}

// undoRetryIfStoppedMeanwhile stops name again after a retried start when the
// instance was meant to stop while it was starting: an anti-affinity hold
// became active (a suppression may have stopped it before this start landed,
// and will not look at it again), or its session was removed by a stop event,
// an expiration or a suppression (nothing would ever stop it otherwise).
func (s *Sablier) undoRetryIfStoppedMeanwhile(ctx context.Context, name string) {
	reason := ""
	if group := s.antiAffinityHold(ctx, name); group != "" {
		reason = fmt.Sprintf("anti-affinity group %q became active", group)
	} else if _, err := s.sessions.Get(ctx, name); errors.Is(err, store.ErrKeyNotFound) {
		reason = "session was removed"
	}
	if reason == "" {
		return
	}
	s.l.InfoContext(ctx, "instance was stopped while starting again, stopping it",
		slog.String("instance", name), slog.String("reason", reason))
	if err := s.provider.InstanceStop(ctx, name); err != nil {
		s.l.ErrorContext(ctx, "cannot stop instance after its retried start",
			slog.String("instance", name), slog.Any("error", err))
	}
}

// instanceName resolves name to the name the provider reports for the
// instance, when the provider accepts several names for one instance. See
// InstanceNameResolver.
func (s *Sablier) instanceName(ctx context.Context, name string) string {
	r, ok := s.provider.(InstanceNameResolver)
	if !ok {
		return name
	}
	if resolved := r.InstanceName(ctx, name); resolved != "" {
		return resolved
	}
	return name
}

func (s *Sablier) InstanceRequest(ctx context.Context, name string, duration time.Duration) (InstanceInfo, error) {
	return s.instanceRequest(ctx, name, duration, false)
}

func (s *Sablier) instanceRequest(ctx context.Context, name string, duration time.Duration, rejectUnlabeled bool) (InstanceInfo, error) {
	if name == "" {
		return InstanceInfo{}, errors.New("instance name cannot be empty")
	}
	name = s.instanceName(ctx, name)

	// Anti-affinity: while one of this instance's antagonist groups holds an
	// active session, it must stay idle. Report it as not-ready with an
	// explanation rather than starting it — which the background reconcile would
	// immediately undo — so blocking callers keep waiting and the waiting page
	// shows why. Once the antagonist expires, a later request proceeds normally.
	if group := s.antiAffinityHold(ctx, name); group != "" {
		s.l.DebugContext(ctx, "instance held by anti-affinity, not starting",
			slog.String("instance", name), slog.String("active_group", group))
		return InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          InstanceStatusNotReady,
			Message:         fmt.Sprintf("paused while group %q is active (anti-affinity)", group),
		}, nil
	}

	newSession := false
	state, err := s.sessions.Get(ctx, name)
	if errors.Is(err, store.ErrKeyNotFound) {
		s.l.DebugContext(ctx, "request to start instance received", slog.String("instance", name))

		state, err = s.requestStart(ctx, name, rejectUnlabeled)
		if err != nil {
			return InstanceInfo{}, err
		}
		newSession = true

		s.l.InfoContext(ctx, "request to start instance dispatched", slog.String("instance", name), slog.String("status", string(state.Status)), slog.Duration("expiration", duration))
	} else if err != nil {
		s.l.ErrorContext(ctx, "request to start instance failed", slog.String("instance", name), slog.Any("error", err))
		return InstanceInfo{}, fmt.Errorf("cannot retrieve instance from store: %w", err)
	} else if retried, ok := s.retryFailedStart(ctx, name, state); ok {
		// The last start failed while the session stayed active. Without this,
		// later requests would only inspect the instance and extend the session
		// until it expires. The recorded failure is used rather than the
		// provider status, which in scale mode reports the idle replicas as
		// ready.
		state = retried
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
		remaining, inWindow, runningHoursErr := runningHoursRemaining(state.RunningHours, state.RunningDays, time.Now())
		if runningHoursErr != nil {
			s.l.WarnContext(ctx, "invalid running-hours or running-days value in state, ignoring", slog.String("instance", name), slog.String("running-hours", state.RunningHours), slog.String("running-days", state.RunningDays), slog.Any("error", runningHoursErr))
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

	// A brand-new session may have made this instance's group(s) active; force any
	// instance that declared an anti-affinity against them to back off.
	if newSession {
		s.triggerAntiAffinityReconcile(ctx)
	}

	return state, nil
}
