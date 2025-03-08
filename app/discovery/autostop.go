package discovery

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"golang.org/x/sync/errgroup"
	"log/slog"
)

// StopAllUnregisteredInstances stops all auto-discovered running instances that are not yet registered
// as running instances by Sablier.
// By default, Sablier does not stop all already running instances. Meaning that you need to make an
// initial request in order to trigger the scaling to zero.
func StopAllUnregisteredInstances(ctx context.Context, p sablier.Provider, s sablier.Store, logger *slog.Logger) error {
	instances, err := p.InstanceList(ctx, provider.InstanceListOptions{
		All:    false, // Only running containers
		Labels: []string{LabelEnable},
	})
	if err != nil {
		return err
	}

	unregistered := make([]string, 0)
	for _, instance := range instances {
		_, err = s.Get(ctx, instance.Name)
		if errors.Is(err, store.ErrKeyNotFound) {
			unregistered = append(unregistered, instance.Name)
		}
	}

	logger.DebugContext(ctx, "found instances to stop", slog.Any("instances", unregistered))

	waitGroup := errgroup.Group{}

	for _, name := range unregistered {
		waitGroup.Go(stopFunc(ctx, name, p, logger))
	}

	return waitGroup.Wait()
}

func stopFunc(ctx context.Context, name string, p sablier.Provider, logger *slog.Logger) func() error {
	return func() error {
		err := p.InstanceStop(ctx, name)
		if err != nil {
			logger.ErrorContext(ctx, "failed to stop instance", slog.String("instance", name), slog.Any("error", err))
			return err
		}
		logger.InfoContext(ctx, "stopped unregistered instance", slog.String("instance", name), slog.String("reason", "instance is enabled but not started by Sablier"))
		return nil
	}
}
