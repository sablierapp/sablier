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
	return s.requestSession(ctx, names, duration, s.rejectUnlabeledRequests)
}

func (s *Sablier) requestSession(ctx context.Context, names []string, duration time.Duration, rejectUnlabeled bool) (sessionState *SessionState, err error) {
	s.l.DebugContext(ctx, "requesting session", slog.Any("names", names), slog.Duration("duration", duration))
	if len(names) == 0 {
		return nil, fmt.Errorf("names cannot be empty")
	}

	var wg sync.WaitGroup

	mx := sync.Mutex{}
	sessionState = &SessionState{
		Instances: map[string]InstanceInfoWithError{},
	}

	wg.Add(len(names))

	for i := 0; i < len(names); i++ {
		go func(name string) {
			defer wg.Done()
			state, err := s.instanceRequest(ctx, name, duration, rejectUnlabeled)
			mx.Lock()
			defer mx.Unlock()
			sessionState.Instances[name] = InstanceInfoWithError{
				Instance: state,
				Error:    err,
			}
		}(names[i])
	}

	wg.Wait()

	return sessionState, nil
}

func (s *Sablier) RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (sessionState *SessionState, err error) {
	s.l.DebugContext(ctx, "requesting session for group", slog.String("group", group), slog.Duration("duration", duration))
	if len(group) == 0 {
		return nil, fmt.Errorf("group is mandatory")
	}

	s.groupsMu.RLock()
	names, ok := s.groups[group]
	available := slices.Collect(maps.Keys(s.groups))
	s.groupsMu.RUnlock()

	if !ok {
		return nil, ErrGroupNotFound{
			Group:           group,
			AvailableGroups: available,
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("group has no member")
	}

	return s.requestSession(ctx, names, duration, false)
}

func (s *Sablier) RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error) {
	return s.requestReadySession(ctx, names, duration, timeout, s.rejectUnlabeledRequests)
}

func (s *Sablier) requestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration, rejectUnlabeled bool) (*SessionState, error) {
	s.l.DebugContext(ctx, "requesting ready session", slog.Any("names", names), slog.Duration("duration", duration), slog.Duration("timeout", timeout))
	session, err := s.requestSession(ctx, names, duration, rejectUnlabeled)
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
				session, err := s.requestSession(ctx, names, duration, rejectUnlabeled)
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

	s.groupsMu.RLock()
	names, ok := s.groups[group]
	deps := s.groupDeps[group]
	s.groupsMu.RUnlock()

	if !ok {
		s.groupsMu.RLock()
		available := slices.Collect(maps.Keys(s.groups))
		s.groupsMu.RUnlock()
		return nil, ErrGroupNotFound{
			Group:           group,
			AvailableGroups: available,
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("group has no member")
	}

	// If the group has no dependency info, fall back to the concurrent approach.
	if len(deps) == 0 {
		return s.requestReadySession(ctx, names, duration, timeout, false)
	}

	// Wave-based startup: start instances in topological order, waiting for each
	// wave to be ready before starting the next wave.
	waves := computeWaves(names, deps)
	if len(waves) <= 1 {
		return s.requestReadySession(ctx, names, duration, timeout, false)
	}

	s.l.DebugContext(ctx, "starting group with dependency ordering",
		slog.String("group", group),
		slog.Int("waves", len(waves)),
	)

	deadline := time.Now().Add(timeout)
	combined := &SessionState{Instances: make(map[string]InstanceInfoWithError)}

	for i, wave := range waves {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("session was not ready after %s", timeout.String())
		}
		s.l.DebugContext(ctx, "starting wave",
			slog.String("group", group),
			slog.Int("wave", i+1),
			slog.Any("instances", wave),
		)
		state, err := s.requestReadySession(ctx, wave, duration, remaining, false)
		if err != nil {
			return nil, err
		}
		for name, info := range state.Instances {
			combined.Instances[name] = info
		}
	}

	return combined, nil
}

// computeWaves partitions names into ordered groups (waves) such that each
// instance appears only after all of its dependencies in deps. Instances with
// no dependencies (or dependencies not listed in names) are placed in Wave 1.
func computeWaves(names []string, deps map[string][]string) [][]string {
	if len(deps) == 0 {
		return [][]string{names}
	}

	satisfied := make(map[string]bool, len(names))
	remaining := make(map[string]bool, len(names))
	for _, n := range names {
		remaining[n] = true
	}

	var waves [][]string
	for len(remaining) > 0 {
		var wave []string
		for _, n := range names { // iterate in original (topo-sorted) order
			if !remaining[n] {
				continue
			}
			allSatisfied := true
			for _, dep := range deps[n] {
				if !satisfied[dep] {
					allSatisfied = false
					break
				}
			}
			if allSatisfied {
				wave = append(wave, n)
			}
		}
		if len(wave) == 0 {
			// Cycle or unresolvable — add all remaining to avoid an infinite loop.
			for n := range remaining {
				wave = append(wave, n)
			}
		}
		for _, n := range wave {
			delete(remaining, n)
			satisfied[n] = true
		}
		waves = append(waves, wave)
	}
	return waves
}
