package sessions

import (
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

type Manager interface {
	RequestSession(names []string, duration time.Duration) *SessionState
	RequestReadySession(names []string, duration time.Duration, timeout time.Duration) (*SessionState, error)

	LoadSessions(io.ReadCloser) error
	SaveSessions(io.WriteCloser) error
}

type SessionsManager struct {
	store    tinykv.KV[instance.State]
	provider providers.Provider
}

func NewSessionsManager(store tinykv.KV[instance.State], provider providers.Provider) Manager {
	return &SessionsManager{
		store:    store,
		provider: provider,
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
	Instance *instance.State
	Error    error
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

func (s *SessionsManager) requestSessionInstance(name string, duration time.Duration) (*instance.State, error) {

	requestState, exists := s.store.Get(name)

	// Trust the stored value
	// TODO: Provider background check on the store
	// Via polling or whatever
	if !exists {
		log.Debugf("starting %s...", name)

		state, err := s.provider.Start(name)

		if err != nil {
			log.Errorf("an error occurred starting %s: %s", name, err.Error())
		}

		requestState.Name = state.Name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.Status = state.Status
		requestState.Message = state.Message
		log.Debugf("status for %s=%s", name, requestState.Status)
	} else if requestState.Status != instance.Ready {
		log.Debugf("checking %s...", name)

		state, err := s.provider.GetState(name)

		if err != nil {
			log.Errorf("an error occurred checking state %s: %s", name, err.Error())
		}

		requestState.Name = state.Name
		requestState.CurrentReplicas = state.CurrentReplicas
		requestState.Status = state.Status
		requestState.Message = state.Message
		log.Debugf("status for %s=%s", name, requestState.Status)
	}

	// Refresh the duration
	s.ExpiresAfter(&requestState, duration)
	return &requestState, nil
}

func (s *SessionsManager) RequestReadySession(names []string, duration time.Duration, timeout time.Duration) (*SessionState, error) {

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
	case status := <-readiness:
		close(quit)
		return status, nil
	case <-time.After(timeout):
		close(quit)
		return nil, fmt.Errorf("session was not ready after %s", timeout.String())
	}
}

func (s *SessionsManager) ExpiresAfter(instance *instance.State, duration time.Duration) {
	s.store.Put(instance.Name, *instance, duration)
}

func (s *SessionState) MarshalJSON() ([]byte, error) {
	instances := []InstanceState{}

	s.Instances.Range(func(key, value interface{}) bool {
		state := value.(InstanceState)
		instances = append(instances, state)
		return true
	})

	return json.Marshal(instances)
}
