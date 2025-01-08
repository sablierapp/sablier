package sablier

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/theme"
	"maps"
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

func NewSablier(ctx context.Context, provider Provider, logger zerolog.Logger) *Sablier {
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

	go s.WatchGroups(ctx, time.Second*5)
	go s.WatchOrphans(ctx, time.Second*5)
	go s.stop(ctx)

	start := time.Now()
	s.updateGroups(ctx)
	total := time.Now().Sub(start)
	s.log.Info().Str("duration", total.String()).Any("groups", s.groups).Msg("initial group scan completed")

	start = time.Now()
	err = s.StopAllUnregistered(ctx)
	total = time.Now().Sub(start)
	if err != nil {
		s.log.Warn().Str("duration", total.String()).Err(err).Msg("an error happened stopping orphans")
	} else {
		s.log.Info().Str("duration", total.String()).Msg("orphans successfully stopped")
	}

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

	diff := MapDiff(s.groups, groups)
	if diff.Changed {
		s.log.Info().
			Any("added", diff.Added).
			Any("removed", diff.Removed).
			Any("groups", s.groups).Msg("groups updated")
	}

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

type MapDiffResult[K comparable] struct {
	Added   []K
	Removed []K
	Changed bool
}

func MapDiff[K comparable, V any](oldMap, newMap map[K]V) MapDiffResult[K] {
	var result MapDiffResult[K]

	for key := range newMap {
		if _, found := oldMap[key]; !found {
			result.Changed = true
			result.Added = append(result.Added, key)
		}
	}

	for key := range oldMap {
		if _, found := newMap[key]; !found {
			result.Changed = true
			result.Removed = append(result.Removed, key)
		}
	}

	return result
}
