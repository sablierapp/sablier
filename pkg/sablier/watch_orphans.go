package sablier

import (
	"context"
	"errors"
	"io"
	"time"
)

func (s *Sablier) WatchOrphans(ctx context.Context, frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	msgs, errs := s.Provider.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errs:
			if !ok {
				s.log.Warn().Msg("WatchOrphans: stopping because provider event stream is closed")
				return
			}
			if errors.Is(err, io.EOF) {
				s.log.Warn().Msg("WatchOrphans: stopping because provider event stream is closed")
				return
			}
			s.log.Warn().Err(err).Msg("WatchOrphans: error received from provider event stream")
		case msg, ok := <-msgs:
			if !ok {
				s.log.Warn().Msg("WatchOrphans: stopping because provider event stream is closed")
				return
			}
			if msg.Action == EventActionStart {
				s.stopIfOrphan(ctx, msg.Instance)
			}
		case <-ticker.C:
			err := s.StopAllUnregistered(ctx)
			if err != nil {
				s.log.Warn().Err(err).Msg("WatchOrphans: error stopping unregistered instances")
			}
		}
	}
}

func (s *Sablier) stopIfOrphan(ctx context.Context, instance InstanceConfig) {
	if !instance.Enabled {
		s.log.Debug().Str("instance", instance.Name).Msg("instance is not enabled, skipping")
		return
	}
	s.pmu.RLock()
	defer s.pmu.RUnlock()
	if s.promises[instance.Name] == nil {
		s.log.Warn().Str("instance", instance.Name).Str("reason", "instance was started by an other source than Sablier").Msg("stopping orphan instance")
		err := s.Provider.Stop(ctx, instance.Name)
		if err != nil {
			s.log.Error().Str("instance", instance.Name).Err(err).Msg("error stopping orphan instance")
		}
	}
}
