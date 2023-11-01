package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/pkg/promise"
	"github.com/acouvreur/sablier/pkg/tinykv"
	log "log/slog"
)

type Manager struct {
	provider provider.Client
	promises map[string]*promise.Promise[Instance]
	timeouts tinykv.KV[string]
	lock     *sync.Mutex
}

func NewManager(p provider.Client, config config.Sessions) *Manager {

	lock := sync.Mutex{}
	promises := make(map[string]*promise.Promise[Instance])
	kv := tinykv.New[string](config.ExpirationInterval, func(k, v string) {
		lock.Lock()
		defer lock.Unlock()
		delete(promises, k)
		err := p.Stop(context.Background(), k)
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
				if msg.Action == provider.EventActionStop {
					manager.deleteSync(msg.Name)
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
