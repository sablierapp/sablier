package sablier

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/theme"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/sablierapp/sablier/pkg/promise"
	"github.com/sablierapp/sablier/pkg/tinykv"
)

type Sablier struct {
	Provider Provider
	Theme    *theme.Themes
	promises map[string]*promise.Promise[InstanceInfo]
	pmu      *sync.RWMutex

	groups map[string][]InstanceConfig
	gmu    *sync.RWMutex

	expirations tinykv.KV[string]

	log zerolog.Logger
}

func NewSablier(ctx context.Context, provider Provider) *Sablier {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().
		Logger()

	pmu := &sync.RWMutex{}
	promises := make(map[string]*promise.Promise[InstanceInfo])

	gmu := &sync.RWMutex{}
	groups := make(map[string][]InstanceConfig)

	expirations := tinykv.New(time.Second, func(k string, _ string) {
		pmu.Lock()
		defer pmu.Unlock()
		logger.Trace().Str("instance", k).Msg("instance expired")
		err := provider.Stop(ctx, k)
		if err != nil {
			logger.Error().Str("instance", k).Err(err).Msg("error stopping instance")
		}
		delete(promises, k)
	})

	// TODO: This should be through the constructor
	t, err := theme.New()
	if err != nil {
		panic(err)
	}

	s := &Sablier{
		Provider:    provider,
		Theme:       t,
		promises:    promises,
		pmu:         pmu,
		groups:      groups,
		gmu:         gmu,
		expirations: expirations,
		log:         logger,
	}

	go s.updateGroups(ctx)
	go s.WatchGroups(ctx, time.Second*5)
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
	return slices.Collect(maps.Keys(s.promises))
}

func (s *Sablier) SetGroups(groups map[string][]InstanceConfig) {
	if groups == nil {
		return
	}
	s.gmu.Lock()
	defer s.gmu.Unlock()
	s.groups = groups
}

func (s *Sablier) GetGroup(group string) ([]InstanceConfig, bool) {
	s.gmu.RLock()
	defer s.gmu.RUnlock()
	instances, ok := s.groups[group]
	return instances, ok
}

func (s *Sablier) Groups() []string {
	s.gmu.RLock()
	defer s.gmu.RUnlock()
	m := s.groups
	k := maps.Keys(m)
	sl := slices.Collect(k)
	return sl
}
