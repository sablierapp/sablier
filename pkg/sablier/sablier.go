package sablier

import (
	"context"
	"sync"
	"time"

	"github.com/sablierapp/sablier/pkg/promise"
	"github.com/sablierapp/sablier/pkg/tinykv"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type Sablier struct {
	Provider Provider
	promises map[string]*promise.Promise[InstanceInfo]
	pmu      *sync.RWMutex

	groups map[string][]InstanceConfig
	gmu    *sync.RWMutex

	expirations tinykv.KV[string]
}

func NewSablier(ctx context.Context, provider Provider) *Sablier {
	pmu := &sync.RWMutex{}
	promises := make(map[string]*promise.Promise[InstanceInfo])

	gmu := &sync.RWMutex{}
	groups := make(map[string][]InstanceConfig)

	expirations := tinykv.New(time.Second, func(k string, _ string) {
		pmu.Lock()
		defer pmu.Unlock()
		log.Printf("instance [%s] expired - removing from promises", k)
		err := provider.Stop(ctx, k)
		if err != nil {
			log.Printf("error stopping instance [%s]: %v", k, err)
		}
		delete(promises, k)
	})

	s := &Sablier{
		Provider:    provider,
		promises:    promises,
		pmu:         pmu,
		groups:      groups,
		gmu:         gmu,
		expirations: expirations,
	}

	go s.stop(ctx)

	return s
}

func (s *Sablier) stop(ctx context.Context) {
	<-ctx.Done()
	s.expirations.Stop()
}

func (s *Sablier) RegisteredInstances() []string {
	s.pmu.RLock()
	defer s.pmu.RUnlock()
	return maps.Keys(s.promises)
}

func (s *Sablier) SetGroups(groups map[string][]InstanceConfig) {
	s.gmu.Lock()
	defer s.gmu.Unlock()
	s.groups = groups
}

func (s *Sablier) GetGroup(group string) ([]InstanceConfig, bool) {
	s.gmu.Lock()
	defer s.gmu.Unlock()
	instances, ok := s.groups[group]
	return instances, ok
}

func (s *Sablier) Groups() any {
	s.gmu.Lock()
	defer s.gmu.Unlock()
	return maps.Keys(s.groups)
}
