package sablier

import (
	"context"
	"errors"
	"io"
	"log"
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
				log.Printf("provider event stream is closed")
				return
			}
			if errors.Is(err, io.EOF) {
				log.Printf("provider event stream closed")
				return
			}
			log.Printf("provider event stream error: %v", err)
		case msg, ok := <-msgs:
			if !ok {
				log.Printf("provider event stream is closed")
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
		log.Printf("error listing instances: %v", err)
	}
	groups := array.GroupByProperty(instances, func(t Instance) string {
		return t.Name
	})
	s.SetGroups(groups)
}
