package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sablierapp/sablier/pkg/store"
	"io"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/providers"
	log "github.com/sirupsen/logrus"
)

//go:generate mockgen -package sessionstest -source=sessions_manager.go -destination=sessionstest/mocks_sessions_manager.go *

type Manager interface {
	RequestSession(names []string, duration time.Duration) (*SessionState, error)
	RequestSessionGroup(group string, duration time.Duration) (*SessionState, error)
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*SessionState, error)

	LoadSessions(io.ReadCloser) error
	SaveSessions(io.WriteCloser) error

	RemoveInstance(name string) error
	SetGroups(groups map[string][]string)

	Stop()
}

type SessionsManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	store    store.Store
	provider providers.Provider
	groups   map[string][]string
}

func NewSessionsManager(store store.Store, provider providers.Provider) Manager {
	ctx, cancel := context.WithCancel(context.Background())

	groups, err := provider.GetGroups(ctx)
	if err != nil {
		groups = make(map[string][]string)
		log.Warn("could not get groups", err)
	}

	sm := &SessionsManager{
		ctx:      ctx,
		cancel:   cancel,
		store:    store,
		provider: provider,
		groups:   groups,
	}

	return sm
}

func (sm *SessionsManager) SetGroups(groups map[string][]string) {
	sm.groups = groups
}

func (sm *SessionsManager) RemoveInstance(name string) error {
	return sm.store.Delete(context.Background(), name)
}

func (sm *SessionsManager) LoadSessions(reader io.ReadCloser) error {
	unmarshaler, ok := sm.store.(json.Unmarshaler)
	defer reader.Close()
	if ok {
		return json.NewDecoder(reader).Decode(unmarshaler)
	}
	return nil
}

func (sm *SessionsManager) SaveSessions(writer io.WriteCloser) error {
	marshaler, ok := sm.store.(json.Marshaler)
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
	Instance *instance.State `json:"instance"`
	Error    error           `json:"error"`
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

func (s *SessionsManager) RequestSession(names []string, duration time.Duration) (sessionState *SessionState, err error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("names cannot be empty")
	}

	var wg sync.WaitGroup

	sessionState = &SessionState{
		Instances: map[string]InstanceState{},
	}

	wg.Add(len(names))

	for i := 0; i < len(names); i++ {
		go func(name string) {
			defer wg.Done()
			state, err := s.requestSessionInstance(name, duration)

			sessionState.Instances[name] = InstanceState{
				Instance: state,
				Error:    err,
			}
		}(names[i])
	}

	wg.Wait()

	return sessionState, nil
}

func (s *SessionsManager) RequestSessionGroup(group string, duration time.Duration) (sessionState *SessionState, err error) {
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

	return s.RequestSession(names, duration)
}

func (s *SessionsManager) requestSessionInstance(name string, duration time.Duration) (*instance.State, error) {
	if name == "" {
		return nil, errors.New("instance name cannot be empty")
	}

	requestState, err := s.store.Get(context.TODO(), name)
	if errors.Is(err, store.ErrKeyNotFound) {
		log.Debugf("starting [%s]...", name)

		err := s.provider.Start(s.ctx, name)
		if err != nil {
			return nil, err
		}

		state, err := s.provider.GetState(s.ctx, name)
		if err != nil {
			return nil, err
		}

		requestState.Name = name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.DesiredReplicas = state.DesiredReplicas
		requestState.Status = state.Status
		requestState.Message = state.Message

		log.Debugf("status for [%s]=[%s]", name, requestState.Status)
	} else if err != nil {
		return nil, fmt.Errorf("cannot retrieve instance from store: %w", err)
	} else if requestState.Status != instance.Ready {
		log.Debugf("checking [%s]...", name)
		state, err := s.provider.GetState(s.ctx, name)
		if err != nil {
			return nil, err
		}

		requestState.Name = state.Name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.DesiredReplicas = state.DesiredReplicas
		requestState.Status = state.Status
		requestState.Message = state.Message
		log.Debugf("status for %s=%s", name, requestState.Status)
	}

	log.Debugf("expiring %+v in %v", requestState, duration)
	// Refresh the duration
	s.ExpiresAfter(&requestState, duration)
	return &requestState, nil
}

func (s *SessionsManager) RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error) {
	session, err := s.RequestSession(names, duration)
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
				session, err := s.RequestSession(names, duration)
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
		log.Debug("request cancelled by user, stopping timeout")
		close(quit)
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

func (s *SessionsManager) ExpiresAfter(instance *instance.State, duration time.Duration) {
	err := s.store.Put(context.TODO(), *instance, duration)
	slog.Default().Warn("could not put instance to store, will not expire", slog.Any("error", err), slog.String("instance", instance.Name))
}

func (s *SessionsManager) Stop() {
	// Stop event listeners
	s.cancel()
}

func (s *SessionState) MarshalJSON() ([]byte, error) {
	instances := maps.Values(s.Instances)

	return json.Marshal(map[string]any{
		"instances": instances,
		"status":    s.Status(),
	})
}
