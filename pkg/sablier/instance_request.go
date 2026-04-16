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

func (s *Sablier) requestStart(ctx context.Context, name string) (InstanceInfo, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	if ps, exists := s.pendingStarts[name]; exists {
		select {
		case <-ps.done:
			// Goroutine completed
			if ps.err != nil {
				// Return the unconsumed error and clear the entry so the next call retries
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

	go func() {
		defer close(ps.done)
		if err := s.provider.InstanceStart(context.Background(), name); err != nil {
			ps.err = err
			s.l.ErrorContext(ctx, "async instance start failed", slog.String("instance", name), slog.Any("error", err))
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
		s.l.DebugContext(ctx, "request to check instance status received", slog.String("instance", name), slog.String("current_status", string(state.Status)))
		state, err = s.provider.InstanceInspect(ctx, name)
		if err != nil {
			return InstanceInfo{}, err
		}
		s.l.DebugContext(ctx, "request to check instance status completed", slog.String("instance", name), slog.String("new_status", string(state.Status)))
	}

	s.l.DebugContext(ctx, "set expiration for instance", slog.String("instance", name), slog.Duration("expiration", duration))

	err = s.sessions.Put(ctx, state, duration)
	if err != nil {
		s.l.Error("could not put instance to store, will not expire", slog.Any("error", err), slog.String("instance", state.Name))
		return InstanceInfo{}, fmt.Errorf("could not put instance to store: %w", err)
	}
	return state, nil
}
