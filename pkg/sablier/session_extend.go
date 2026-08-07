package sablier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sablierapp/sablier/pkg/store"
)

// ExtendSession extends the TTL of already-tracked instances in the session
// store. It does NOT start any instance; if a named instance is not currently
// tracked, it is reported with a store.ErrKeyNotFound error in the result.
func (s *Sablier) ExtendSession(ctx context.Context, names []string, duration time.Duration) (*SessionState, error) {
	s.l.DebugContext(ctx, "extending session", slog.Any("names", names), slog.Duration("duration", duration))
	if len(names) == 0 {
		return nil, fmt.Errorf("names cannot be empty")
	}

	var (
		wg           sync.WaitGroup
		mx           sync.Mutex
		sessionState = &SessionState{Instances: make(map[string]InstanceInfoWithError, len(names))}
	)

	wg.Add(len(names))
	for _, name := range names {
		go func(n string) {
			defer wg.Done()
			result := s.extendOne(ctx, n, duration)
			mx.Lock()
			sessionState.Instances[n] = result
			mx.Unlock()
		}(name)
	}
	wg.Wait()

	return sessionState, nil
}

// ExtendSessionGroup extends the TTL of all instances belonging to a named
// group. Returns ErrGroupNotFound when the group is unknown.
func (s *Sablier) ExtendSessionGroup(ctx context.Context, group string, duration time.Duration) (*SessionState, error) {
	s.l.DebugContext(ctx, "extending session for group", slog.String("group", group), slog.Duration("duration", duration))
	if group == "" {
		return nil, fmt.Errorf("group is mandatory")
	}

	names, ok := s.groups.Get(group)
	if !ok {
		return nil, ErrGroupNotFound{
			Group:           group,
			AvailableGroups: s.groups.Keys(),
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("group has no members")
	}

	return s.ExtendSession(ctx, names, duration)
}

// extendOne refreshes the TTL for a single instance. If the instance is not
// in the store the error field contains store.ErrKeyNotFound.
func (s *Sablier) extendOne(ctx context.Context, name string, duration time.Duration) InstanceInfoWithError {
	info, err := s.sessions.Get(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			return InstanceInfoWithError{Error: fmt.Errorf("instance %q is not tracked by Sablier: %w", name, store.ErrKeyNotFound)}
		}
		return InstanceInfoWithError{Error: err}
	}

	if err := s.sessions.Put(ctx, info, duration); err != nil {
		return InstanceInfoWithError{Error: fmt.Errorf("could not extend session for %q: %w", name, err)}
	}

	return InstanceInfoWithError{Instance: info}
}
