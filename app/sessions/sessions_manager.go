package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/sablierapp/sablier/pkg/store"
	"io"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/providers"
)

//go:generate mockgen -package sessionstest -source=sessions_manager.go -destination=sessionstest/mocks_sessions_manager.go *

type Manager interface {
	RequestSession(ctx context.Context, names []string, duration time.Duration) (*SessionState, error)
	RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (*SessionState, error)
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*SessionState, error)

	LoadSessions(io.ReadCloser) error
	SaveSessions(io.WriteCloser) error

	RemoveInstance(name string) error
	SetGroups(groups map[string][]string)
}

type SessionsManager struct {
	store    store.Store
	provider providers.Provider
	groups   map[string][]string

	l *slog.Logger
}

func NewSessionsManager(logger *slog.Logger, store store.Store, provider providers.Provider) Manager {
	sm := &SessionsManager{
		store:    store,
		provider: provider,
		groups:   map[string][]string{},
		l:        logger,
	}

	return sm
}

func (s *SessionsManager) SetGroups(groups map[string][]string) {
	if groups == nil {
		groups = map[string][]string{}
	}
	if diff := cmp.Diff(s.groups, groups); diff != "" {
		// TODO: Change this log for a friendly logging, groups rarely change, so we can put some effort on displaying what changed
		s.l.Info("set groups", slog.Any("old", s.groups), slog.Any("new", groups), slog.Any("diff", diff))
		s.groups = groups
	}
}

func (s *SessionsManager) RemoveInstance(name string) error {
	return s.store.Delete(context.Background(), name)
}

func (s *SessionsManager) LoadSessions(reader io.ReadCloser) error {
	unmarshaler, ok := s.store.(json.Unmarshaler)
	defer reader.Close()
	if ok {
		return json.NewDecoder(reader).Decode(unmarshaler)
	}
	return nil
}

func (s *SessionsManager) SaveSessions(writer io.WriteCloser) error {
	marshaler, ok := s.store.(json.Marshaler)
	defer writer.Close()
	if ok {
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")

		return encoder.Encode(marshaler)
	}
	return nil
}

type InstanceState struct {
	Instance instance.State `json:"instance"`
	Error    error          `json:"error"`
}

type SessionState struct {
	Instances map[string]InstanceState `json:"instances"`
}

func (s *SessionState) IsReady() bool {
	if s.Instances == nil {
		s.Instances = map[string]InstanceState{}
	}

	for _, v := range s.Instances {
		if v.Error != nil || v.Instance.Status != instance.Ready {
			return false
		}
	}

	return true
}

func (s *SessionState) Status() string {
	if s.IsReady() {
		return "ready"
	}

	return "not-ready"
}

func (s *SessionsManager) RequestSession(ctx context.Context, names []string, duration time.Duration) (sessionState *SessionState, err error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("names cannot be empty")
	}

	var wg sync.WaitGroup

	mx := sync.Mutex{}
	sessionState = &SessionState{
		Instances: map[string]InstanceState{},
	}

	wg.Add(len(names))

	for i := 0; i < len(names); i++ {
		go func(name string) {
			defer wg.Done()
			state, err := s.requestInstance(ctx, name, duration)
			mx.Lock()
			defer mx.Unlock()
			sessionState.Instances[name] = InstanceState{
				Instance: state,
				Error:    err,
			}
		}(names[i])
	}

	wg.Wait()

	return sessionState, nil
}

func (s *SessionsManager) RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (sessionState *SessionState, err error) {
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

func (s *SessionsManager) requestInstance(ctx context.Context, name string, duration time.Duration) (instance.State, error) {
	if name == "" {
		return instance.State{}, errors.New("instance name cannot be empty")
	}

	state, err := s.store.Get(ctx, name)
	if errors.Is(err, store.ErrKeyNotFound) {
		s.l.DebugContext(ctx, "request to start instance received", slog.String("instance", name))

		err := s.provider.Start(ctx, name)
		if err != nil {
			return instance.State{}, err
		}

		state, err = s.provider.GetState(ctx, name)
		if err != nil {
			return instance.State{}, err
		}
		s.l.DebugContext(ctx, "request to start instance status completed", slog.String("instance", name), slog.String("status", state.Status))
	} else if err != nil {
		s.l.ErrorContext(ctx, "request to start instance failed", slog.String("instance", name), slog.Any("error", err))
		return instance.State{}, fmt.Errorf("cannot retrieve instance from store: %w", err)
	} else if state.Status != instance.Ready {
		s.l.DebugContext(ctx, "request to check instance status received", slog.String("instance", name), slog.String("current_status", state.Status))
		state, err = s.provider.GetState(ctx, name)
		if err != nil {
			return instance.State{}, err
		}
		s.l.DebugContext(ctx, "request to check instance status completed", slog.String("instance", name), slog.String("new_status", state.Status))
	}

	s.l.DebugContext(ctx, "set expiration for instance", slog.String("instance", name), slog.Duration("expiration", duration))
	// Refresh the duration
	s.expiresAfter(ctx, state, duration)
	return state, nil
}

func (s *SessionsManager) RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error) {
	session, err := s.RequestSession(ctx, names, duration)
	if err != nil {
		return nil, err
	}

	if session.IsReady() {
		return session, nil
	}

	ticker := time.NewTicker(5 * time.Second)
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

func (s *SessionsManager) RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (sessionState *SessionState, err error) {

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

func (s *SessionsManager) expiresAfter(ctx context.Context, instance instance.State, duration time.Duration) {
	err := s.store.Put(ctx, instance, duration)
	if err != nil {
		s.l.Error("could not put instance to store, will not expire", slog.Any("error", err), slog.String("instance", instance.Name))
	}
}

func (s *SessionState) MarshalJSON() ([]byte, error) {
	instances := maps.Values(s.Instances)

	return json.Marshal(map[string]any{
		"instances": instances,
		"status":    s.Status(),
	})
}
