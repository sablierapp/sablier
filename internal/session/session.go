package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/pkg/promise"
	"github.com/acouvreur/sablier/pkg/tinykv"
	log "log/slog"
)

type Session struct {
	ExpiresOn       time.Time
	SessionDuration time.Duration
	StartedAt       time.Time
}

func NewSession(duration time.Duration) Session {
	return Session{
		ExpiresOn:       time.Now().Add(duration),
		SessionDuration: duration,
		StartedAt:       time.Now(),
	}
}

type Manager struct {
	provider provider.Client
	promises map[string]*promise.Promise[Instance]
	timeouts tinykv.KV[Session]
	lock     *sync.Mutex
}

func NewManager(p provider.Client, config config.Sessions) *Manager {

	lock := sync.Mutex{}
	promises := make(map[string]*promise.Promise[Instance])
	kv := tinykv.New[Session](config.ExpirationInterval, func(k string, v Session) {
		lock.Lock()
		defer lock.Unlock()
		delete(promises, k)
		err := p.Stop(context.Background(), k)
		log.Info("stopping instance", "instance", k)
		if err != nil {
			log.Warn(fmt.Sprintf("stopping %s: ", k), err)
		}
	})

	manager := Manager{
		provider: p,
		promises: promises,
		timeouts: kv,
		lock:     &lock,
	}

	msgs, errs := p.Events(context.Background())
	started := make(chan struct{})
	go func() {
		close(started)
		for {
			select {
			case msg := <-msgs:
				if msg.Action == provider.EventActionStop || msg.Action == provider.EventActionDestroy {
					if _, ok := promises[msg.Name]; ok {
						log.Info("instance was stopped from external source", "instance", msg.Name)
						manager.deleteSync(msg.Name)
						kv.Delete(msg.Name)
					}
				}
			case err := <-errs:
				if errors.Is(err, io.EOF) {
					return
				}
				log.Warn("event: ", err)
			}
		}
	}()
	<-started

	return &manager
}

func (s *Manager) deleteSync(k string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.promises, k)
}
