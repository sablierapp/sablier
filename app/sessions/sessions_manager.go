package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/pkg/tinykv"
	log "github.com/sirupsen/logrus"
)

const defaultRefreshFrequency = 2 * time.Second

type Manager interface {
	RequestSession(names []string, duration time.Duration) *SessionState
	RequestSessionGroup(group string, duration time.Duration) *SessionState
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*SessionState, error)

	LoadSessions(io.ReadCloser) error
	SaveSessions(io.WriteCloser) error

	Stop()
}

type SessionsManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	store    tinykv.KV[instance.State]
	provider providers.Provider
	groups   map[string][]string
}

func NewSessionsManager(store tinykv.KV[instance.State], provider providers.Provider) Manager {
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

	sm.initWatchers()

	return sm
}

func (sm *SessionsManager) initWatchers() {
	updateGroups := make(chan map[string][]string)
	go watchGroups(sm.ctx, sm.provider, defaultRefreshFrequency, updateGroups)
	go sm.consumeGroups(updateGroups)

	instanceStopped := make(chan string)
	go sm.provider.NotifyInstanceStopped(sm.ctx, instanceStopped)
	go sm.consumeInstanceStopped(instanceStopped)
}

func (sm *SessionsManager) consumeGroups(receive chan map[string][]string) {
	for groups := range receive {
		sm.groups = groups
	}
}

func (sm *SessionsManager) consumeInstanceStopped(instanceStopped chan string) {
	for instance := range instanceStopped {
		// Will delete from the store containers that have been stop either by external sources
		// or by the internal expiration loop, if the deleted entry does not exist, it doesn't matter
		log.Debugf("received event instance %s is stopped, removing from store", instance)
		sm.store.Delete(instance)
	}
}

func (sm *SessionsManager) LoadSessions(reader io.ReadCloser) error {
	defer reader.Close()
	return json.NewDecoder(reader).Decode(sm.store)
}

func (sm *SessionsManager) SaveSessions(writer io.WriteCloser) error {
	defer writer.Close()

	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	return encoder.Encode(sm.store)
}

type InstanceState struct {
	Instance *instance.State `json:"instance"`
	Error    error           `json:"error"`
}

type SessionState struct {
	Instances *sync.Map
}

func (s *SessionState) IsReady() bool {
	ready := true

	s.Instances.Range(func(key, value interface{}) bool {
		state := value.(InstanceState)
		if state.Error != nil || state.Instance.Status != instance.Ready {
			ready = false
			return false
		}
		return true
	})

	return ready
}

func (s *SessionState) Status() string {
	if s.IsReady() {
		return "ready"
	}

	return "not-ready"
}

func (s *SessionsManager) RequestSession(names []string, duration time.Duration) (sessionState *SessionState) {

	if len(names) == 0 {
		return nil
	}

	var wg sync.WaitGroup

	sessionState = &SessionState{
		Instances: &sync.Map{},
	}

	wg.Add(len(names))

	for i := 0; i < len(names); i++ {
		go func(name string) {
			defer wg.Done()
			state, err := s.requestSessionInstance(name, duration)

			sessionState.Instances.Store(name, InstanceState{
				Instance: state,
				Error:    err,
			})
		}(names[i])
	}

	wg.Wait()

	return sessionState
}

func (s *SessionsManager) RequestSessionGroup(group string, duration time.Duration) (sessionState *SessionState) {

	if len(group) == 0 {
		return nil
	}

	names := s.groups[group]

	if len(names) == 0 {
		return nil
	}

	return s.RequestSession(names, duration)
}

func (s *SessionsManager) requestSessionInstance(name string, duration time.Duration) (*instance.State, error) {

	requestState, exists := s.store.Get(name)

	if !exists {
		log.Debugf("starting %s...", name)

		state, err := s.provider.Start(s.ctx, name)

		if err != nil {
			log.Errorf("an error occurred starting %s: %s", name, err.Error())
		}

		requestState.Name = state.Name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.DesiredReplicas = state.DesiredReplicas
		requestState.Status = state.Status
		requestState.Message = state.Message
		log.Debugf("status for %s=%s", name, requestState.Status)
	} else if requestState.Status != instance.Ready {
		log.Debugf("checking %s...", name)

		state, err := s.provider.GetState(s.ctx, name)

		if err != nil {
			log.Errorf("an error occurred checking state %s: %s", name, err.Error())
		}

		requestState.Name = state.Name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.DesiredReplicas = state.DesiredReplicas
		requestState.Status = state.Status
		requestState.Message = state.Message
		log.Debugf("status for %s=%s", name, requestState.Status)
	}

	// Refresh the duration
	s.ExpiresAfter(&requestState, duration)
	return &requestState, nil
}

func (s *SessionsManager) RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error) {

	session := s.RequestSession(names, duration)
	if session.IsReady() {
		return session, nil
	}

	ticker := time.NewTicker(5 * time.Second)
	readiness := make(chan *SessionState)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				session := s.RequestSession(names, duration)
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
	case <-time.After(timeout):
		close(quit)
		return nil, fmt.Errorf("session was not ready after %s", timeout.String())
	}
}

func (s *SessionsManager) RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (sessionState *SessionState, err error) {

	if len(group) == 0 {
		return nil, fmt.Errorf("group is mandatory")
	}

	names := s.groups[group]

	if len(names) == 0 {
		return nil, fmt.Errorf("group has no member")
	}

	return s.RequestReadySession(ctx, names, duration, timeout)
}

func (s *SessionsManager) ExpiresAfter(instance *instance.State, duration time.Duration) {
	s.store.Put(instance.Name, *instance, duration)
}

func (s *SessionsManager) Stop() {
	// Stop event listeners
	s.cancel()

	// Stop the store
	s.store.Stop()
}

func (s *SessionState) MarshalJSON() ([]byte, error) {
	instances := []InstanceState{}

	s.Instances.Range(func(key, value interface{}) bool {
		state := value.(InstanceState)
		instances = append(instances, state)
		return true
	})

	return json.Marshal(map[string]any{
		"instances": instances,
		"status":    s.Status(),
	})
}
