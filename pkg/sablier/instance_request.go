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
}

// consumePendingError checks whether a pending start exists for the given
// instance. It returns (pending, error):
//   - pending=true, err=nil  → start still in progress, caller should skip inspect
//   - pending=false, err!=nil → start completed with error, entry cleared for retry
//   - pending=false, err=nil  → no pending entry or already cleaned up
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

	// Detach from the request context to avoid retaining HTTP request values,
	// but use a bounded timeout to prevent goroutine leaks.
	startCtx, cancel := context.WithTimeout(context.Background(), s.InstanceStartTimeout)

	go func() {
		defer cancel()
		defer close(ps.done)
		if err := s.provider.InstanceStart(startCtx, name); err != nil {
			ps.err = err
			s.l.Error("async instance start failed", slog.String("instance", name), slog.Any("error", err))
		} else {
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

		s.l.DebugContext(ctx, "request to start instance dispatched", slog.String("instance", name), slog.String("status", string(state.Status)))
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
			s.l.DebugContext(ctx, "request to check instance status completed", slog.String("instance", name), slog.String("new_status", string(state.Status)))
		}
	}

	s.l.DebugContext(ctx, "set expiration for instance", slog.String("instance", name), slog.Duration("expiration", duration))

	err = s.sessions.Put(ctx, state, duration)
	if err != nil {
		s.l.Error("could not put instance to store, will not expire", slog.Any("error", err), slog.String("instance", state.Name))
		return InstanceInfo{}, fmt.Errorf("could not put instance to store: %w", err)
	}
	return state, nil
}
