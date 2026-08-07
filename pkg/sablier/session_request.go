package sablier

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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

	for i := range names {
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

	names, ok := s.groups.Get(group)
	if !ok {
		return nil, ErrGroupNotFound{
			Group:           group,
			AvailableGroups: s.groups.Keys(),
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
	// Buffered capacity 1: the goroutine can complete its send even when the
	// outer select has already chosen a different case (timeout, cancellation).
	// Without the buffer the goroutine would block forever on the channel send.
	readiness := make(chan *SessionState, 1)
	errch := make(chan error, 1)
	quit := make(chan struct{})

	// last publishes the most recent not-ready session so the timeout branch can
	// report which instances were still pending, and why. Each poll produces a
	// fresh SessionState, so the loaded value is never mutated concurrently.
	var last atomic.Pointer[SessionState]
	last.Store(session)

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
				last.Store(session)
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
		timeoutErr := ErrTimeout{Duration: timeout}
		if ls := last.Load(); ls != nil {
			timeoutErr.Instances = ls.NotReadyInstances()
		}
		return nil, timeoutErr
	}
}

func (s *Sablier) RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (sessionState *SessionState, err error) {
	s.l.DebugContext(ctx, "requesting ready session for group", slog.String("group", group), slog.Duration("duration", duration), slog.Duration("timeout", timeout))
	if len(group) == 0 {
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
		return nil, fmt.Errorf("group has no member")
	}

	return s.requestReadySession(ctx, names, duration, timeout, false)
}
