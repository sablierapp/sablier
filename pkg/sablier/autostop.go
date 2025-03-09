package sablier

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/store"
	"golang.org/x/sync/errgroup"
	"log/slog"
)

// StopAllUnregisteredInstances stops all auto-discovered running instances that are not yet registered
// as running instances by Sablier.
// By default, Sablier does not stop all already running instances. Meaning that you need to make an
// initial request in order to trigger the scaling to zero.
func (s *sablier) StopAllUnregisteredInstances(ctx context.Context) error {
	instances, err := s.provider.InstanceList(ctx, provider.InstanceListOptions{
		All: false, // Only running instances
	})
	if err != nil {
		return err
	}

	unregistered := make([]string, 0)
	for _, instance := range instances {
		_, err = s.sessions.Get(ctx, instance.Name)
		if errors.Is(err, store.ErrKeyNotFound) {
			unregistered = append(unregistered, instance.Name)
		}
	}

	s.l.DebugContext(ctx, "found instances to stop", slog.Any("instances", unregistered))

	waitGroup := errgroup.Group{}

	for _, name := range unregistered {
		waitGroup.Go(s.stopFunc(ctx, name))
	}

	return waitGroup.Wait()
}

func (s *sablier) stopFunc(ctx context.Context, name string) func() error {
	return func() error {
		err := s.provider.InstanceStop(ctx, name)
		if err != nil {
			s.l.ErrorContext(ctx, "failed to stop instance", slog.String("instance", name), slog.Any("error", err))
			return err
		}
		s.l.InfoContext(ctx, "stopped unregistered instance", slog.String("instance", name), slog.String("reason", "instance is enabled but not started by Sablier"))
		return nil
	}
}
