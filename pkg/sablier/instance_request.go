package sablier

import (
	"context"
	"errors"
	"fmt"
	"github.com/sablierapp/sablier/pkg/store"
	"log/slog"
	"time"
)

func (s *sablier) InstanceRequest(ctx context.Context, name string, duration time.Duration) (InstanceInfo, error) {
	if name == "" {
		return InstanceInfo{}, errors.New("instance name cannot be empty")
	}

	state, err := s.sessions.Get(ctx, name)
	if errors.Is(err, store.ErrKeyNotFound) {
		s.l.DebugContext(ctx, "request to start instance received", slog.String("instance", name))

		err = s.provider.InstanceStart(ctx, name)
		if err != nil {
			return InstanceInfo{}, err
		}

		state, err = s.provider.InstanceInspect(ctx, name)
		if err != nil {
			return InstanceInfo{}, err
		}
		s.l.DebugContext(ctx, "request to start instance status completed", slog.String("instance", name), slog.String("status", string(state.Status)))
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
