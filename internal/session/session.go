package session

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/pkg/promise"
	"github.com/acouvreur/sablier/pkg/tinykv"
)

type SessionManager struct {
	provider  provider.Client
	promises  map[string]*promise.Promise[Instance]
	instances tinykv.KV[string]
	lock      *sync.Mutex
}

func NewSessionManager(p provider.Client, config config.Sessions) *SessionManager {

	lock := sync.Mutex{}
	promises := make(map[string]*promise.Promise[Instance])
	kv := tinykv.New[string](config.ExpirationInterval, func(k, v string) {
		lock.Lock()
		defer lock.Unlock()
		delete(promises, k)
		p.Stop(context.Background(), k)
	})

	manager := SessionManager{
		provider:  p,
		promises:  promises,
		instances: kv,
		lock:      &lock,
	}

	msgs, errs := p.Events(context.Background())
	started := make(chan struct{})
	go func() {
		close(started)
		for {
			select {
			case msg := <-msgs:
				if msg.Action == provider.EventActionStop {
					manager.deleteSync(msg.Name)
				}
			case err := <-errs:
				if errors.Is(err, io.EOF) {
					return
				}
			}
		}
	}()
	<-started

	return &manager
}

func (s *SessionManager) deleteSync(k string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.promises, k)
}
