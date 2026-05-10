package sablier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sablierapp/sablier/pkg/store"
)

type pendingStart struct {
	done chan struct{}
	err  error
	info InstanceInfo // set at creation time; used to return consistent data while starting
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

func (s *Sablier) requestStart(ctx context.Context, name string) (InstanceInfo, error) {
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
			info := ps.info
			s.pendingMu.Unlock()
			return info, nil
		}
	}
	s.pendingMu.Unlock()

	// Inspect outside the lock: this may be a slow remote network call.
	// If inspect fails (e.g. first boot), fall back to a minimal struct so
	// the start still proceeds.
	info, err := s.provider.InstanceInspect(ctx, name)
	if err != nil {
		s.l.DebugContext(ctx, "pre-start inspect failed, using bare info", slog.String("instance", name), slog.Any("error", err))
		info = InstanceInfo{Name: name, CurrentReplicas: 0, DesiredReplicas: 1}
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
			existingInfo := existing.info
			s.pendingMu.Unlock()
			return existingInfo, nil
		}
	}
	ps := &pendingStart{done: make(chan struct{}), info: info}
	s.pendingStarts[name] = ps
	s.pendingMu.Unlock()

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

	return info, nil
}

func (s *Sablier) InstanceRequest(ctx context.Context, name string, duration time.Duration) (InstanceInfo, error) {
	if name == "" {
		return InstanceInfo{}, errors.New("instance name cannot be empty")
	}

	state, err := s.sessions.Get(ctx, name)
	if errors.Is(err, store.ErrKeyNotFound) {
		s.l.DebugContext(ctx, "request to start instance received", slog.String("instance", name))

		state, err = s.requestStart(ctx, name)
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
			state, err = s.provider.InstanceInspect(ctx, name)
			if err != nil {
				return InstanceInfo{}, err
			}
			if state.Status == InstanceStatusReady {
				s.metrics.RecordReadyWaitEnd(name)
			}
			s.l.DebugContext(ctx, "request to check instance status completed", slog.String("instance", name), slog.String("new_status", string(state.Status)))
		}
	}

	s.l.DebugContext(ctx, "set expiration for instance", slog.String("instance", name), slog.Duration("expiration", duration))

	err = s.sessions.Put(ctx, state, duration)
	if err != nil {
		s.l.ErrorContext(ctx, "could not put instance to store, will not expire", slog.Any("error", err), slog.String("instance", state.Name))
		return InstanceInfo{}, fmt.Errorf("could not put instance to store: %w", err)
	}
	return state, nil
}
