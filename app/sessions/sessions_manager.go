package sessions

import (
	"sync"
	"time"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/pkg/tinykv"
	log "github.com/sirupsen/logrus"
)

type Manager interface {
	RequestSession(names []string, duration time.Duration) *SessionState
	RequestReadySession(names []string, duration time.Duration, timeout time.Duration) *SessionState
}

type SessionsManager struct {
	store    tinykv.KV[instance.State]
	provider providers.Provider
}

func NewSessionsManager(conf config.Sessions, provider providers.Provider) (Manager, error) {

	store := tinykv.New(conf.ExpirationInterval, onSessionExpires(provider))

	return &SessionsManager{
		store:    store,
		provider: provider,
	}, nil
}

type InstanceState struct {
	Instance *instance.State
	Error    error
}

type SessionState struct {
	Instances *sync.Map
}

func onSessionExpires(provider providers.Provider) func(key string, instance instance.State) {
	return func(key string, instance instance.State) {
		log.Debugf("stopping %s...", key)
		_, err := provider.Stop(key)

		if err != nil {
			log.Warnf("error stopping %s: %s", key, err.Error())
		} else {
			log.Debugf("stopped %s", key)
		}
	}
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

func (s *SessionsManager) RequestSession(names []string, duration time.Duration) (sessionState *SessionState) {

	var wg sync.WaitGroup

	wg.Add(len(names))

	for i := 0; i < len(names); i++ {
		name := names[i]
		go func() {
			defer wg.Done()
			state, err := s.requestSessionInstance(name, duration)

			sessionState.Instances.Store(name, InstanceState{
				Instance: state,
				Error:    err,
			})
		}()
	}

	wg.Wait()

	return sessionState
}

func (s *SessionsManager) requestSessionInstance(name string, duration time.Duration) (*instance.State, error) {

	requestState, exists := s.store.Get(name)

	// Trust the stored value
	// TODO: Provider background check on the store
	// Via polling or whatever
	if !exists || requestState.Status != instance.Ready {
		state, err := s.provider.Start(name)

		if err != nil {
			return nil, err
		}

		requestState.Name = state.Name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.Status = state.Status
		requestState.Error = state.Error
	}

	// Refresh the duration
	s.ExpiresAfter(&requestState, duration)
	return &requestState, nil
}

func (s *SessionsManager) RequestReadySession(names []string, duration time.Duration, timeout time.Duration) *SessionState {
	return nil
}

func (s *SessionsManager) ExpiresAfter(request *instance.State, duration time.Duration) {
	s.store.Put(request.Name, *request, tinykv.ExpiresAfter(duration))
}
