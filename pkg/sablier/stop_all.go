package sablier

import (
	"context"

	"github.com/sablierapp/sablier/pkg/array"
	"github.com/sablierapp/sablier/pkg/provider"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func (s *Sablier) StopAllUnregistered(ctx context.Context) error {
	instances, err := s.Provider.List(ctx, provider.ListOptions{
		All: false,
	})
	if err != nil {
		return err
	}

	registered := s.RegisteredInstances()
	unregistered := array.RemoveElements(instances, registered)
	log.Tracef("Found %v unregistered instances ", len(unregistered))

	waitGroup := errgroup.Group{}

	// Previously, the variables declared by a “for” loop were created once and updated by each iteration.
	// In Go 1.22, each iteration of the loop creates new variables, to avoid accidental sharing bugs.
	// The transition support tooling described in the proposal continues to work in the same way it did in Go 1.21.
	for _, name := range unregistered {
		waitGroup.Go(func() error {
			return s.Provider.Stop(ctx, name)
		})
	}

	return waitGroup.Wait()
}
