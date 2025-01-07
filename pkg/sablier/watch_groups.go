package sablier

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/sablierapp/sablier/pkg/array"
	"github.com/sablierapp/sablier/pkg/provider"
)

func (s *Sablier) WatchGroups(ctx context.Context, frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	msgs, errs := s.Provider.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errs:
			if !ok {
				s.log.Warn().Msg("WatchGroups: stopping because provider event stream is closed")
				return
			}
			if errors.Is(err, io.EOF) {
				s.log.Warn().Msg("WatchGroups: stopping because provider event stream is closed")
				return
			}
			s.log.Warn().Err(err).Msg("WatchGroups: error received from provider event stream")
		case msg, ok := <-msgs:
			if !ok {
				s.log.Warn().Msg("WatchGroups: stopping because provider event stream is closed")
				return
			}
			if msg.Action == EventActionCreate || msg.Action == EventActionRemove {
				s.updateGroups(ctx)
			}
		case <-ticker.C:
			s.updateGroups(ctx)
		}
	}
}

func (s *Sablier) updateGroups(ctx context.Context) {
	instances, err := s.Provider.List(ctx, provider.ListOptions{All: true})
	if err != nil {
		s.log.Error().Err(err).Msg("cannot update group: error listing instances")
		return
	}
	groups := array.GroupByProperty(instances, func(t InstanceConfig) string {
		return t.Name
	})
	s.SetGroups(groups)
}
