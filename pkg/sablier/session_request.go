package sablier

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"
)

type InstanceInfoWithError struct {
	Instance InstanceInfo `json:"instance"`
	Error    error        `json:"error"`
}

func (s *Sablier) RequestSession(ctx context.Context, names []string, duration time.Duration) (sessionState *SessionState, err error) {
	s.l.DebugContext(ctx, "requesting session", slog.Any("names", names), slog.Duration("duration", duration))
	if len(names) == 0 {
		return nil, fmt.Errorf("names cannot be empty")
	}

	// Resolve dependencies and compute start order.
	deps := s.GetDependencies()
	ordered, err := s.resolveStartOrder(names, deps)
	if err != nil {
		s.l.WarnContext(ctx, "failed to resolve dependency order, falling back to parallel start", slog.Any("error", err))
		ordered = names
	}

	s.l.DebugContext(ctx, "resolved start order", slog.Any("order", ordered))

	var wg sync.WaitGroup

	mx := sync.Mutex{}
	sessionState = &SessionState{
		Instances: map[string]InstanceInfoWithError{},
	}

	wg.Add(len(ordered))

	for i := 0; i < len(ordered); i++ {
		go func(name string) {
			defer wg.Done()
			state, err := s.InstanceRequest(ctx, name, duration)
			mx.Lock()
			defer mx.Unlock()
			sessionState.Instances[name] = InstanceInfoWithError{
				Instance: state,
				Error:    err,
			}
		}(ordered[i])
	}

	wg.Wait()

	return sessionState, nil
}

// resolveStartOrder takes a list of requested instance names and a dependency
// graph, resolves all transitive dependencies, deduplicates, and returns them
// in topological order (deepest dependencies first).
func (s *Sablier) resolveStartOrder(names []string, deps map[string][]string) ([]string, error) {
	if len(deps) == 0 {
		return names, nil
	}

	seen := make(map[string]bool)
	var ordered []string

	for _, name := range names {
		resolved, err := ResolveDependencyOrder(name, deps)
		if err != nil {
			return nil, err
		}
		for _, r := range resolved {
			if !seen[r] {
				seen[r] = true
				ordered = append(ordered, r)
			}
		}
	}

	return ordered, nil
}

func (s *Sablier) RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (sessionState *SessionState, err error) {
	s.l.DebugContext(ctx, "requesting session for group", slog.String("group", group), slog.Duration("duration", duration))
	if len(group) == 0 {
		return nil, fmt.Errorf("group is mandatory")
	}

	names, ok := s.groups[group]
	if !ok {
		return nil, ErrGroupNotFound{
			Group:           group,
			AvailableGroups: slices.Collect(maps.Keys(s.groups)),
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("group has no member")
	}

	return s.RequestSession(ctx, names, duration)
}

func (s *Sablier) RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error) {
	s.l.DebugContext(ctx, "requesting ready session", slog.Any("names", names), slog.Duration("duration", duration), slog.Duration("timeout", timeout))
	session, err := s.RequestSession(ctx, names, duration)
	if err != nil {
		return nil, err
	}

	if session.IsReady() {
		return session, nil
	}

	if err := session.InstanceErrors(); err != nil {
		return nil, err
	}

	ticker := time.NewTicker(s.BlockingRefreshFrequency)
	readiness := make(chan *SessionState)
	errch := make(chan error)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				session, err := s.RequestSession(ctx, names, duration)
				if err != nil {
					errch <- err
					return
				}
				if session.IsReady() {
					readiness <- session
					return
				}
				if err := session.InstanceErrors(); err != nil {
					errch <- err
					return
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		s.l.DebugContext(ctx, "request cancelled", slog.Any("reason", ctx.Err()))
		close(quit)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request cancelled by user: %w", ctx.Err())
		}
		return nil, fmt.Errorf("request cancelled by user")
	case status := <-readiness:
		close(quit)
		return status, nil
	case err := <-errch:
		close(quit)
		return nil, err
	case <-time.After(timeout):
		close(quit)
		return nil, fmt.Errorf("session was not ready after %s", timeout.String())
	}
}

func (s *Sablier) RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (sessionState *SessionState, err error) {
	s.l.DebugContext(ctx, "requesting ready session for group", slog.String("group", group), slog.Duration("duration", duration), slog.Duration("timeout", timeout))
	if len(group) == 0 {
		return nil, fmt.Errorf("group is mandatory")
	}

	names, ok := s.groups[group]
	if !ok {
		return nil, ErrGroupNotFound{
			Group:           group,
			AvailableGroups: slices.Collect(maps.Keys(s.groups)),
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("group has no member")
	}

	return s.RequestReadySession(ctx, names, duration, timeout)
}
