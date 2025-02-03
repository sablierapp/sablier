package discovery

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/app/providers"
	"github.com/sablierapp/sablier/pkg/store"
	"golang.org/x/sync/errgroup"
	"log/slog"
)

// StopAllUnregisteredInstances stops all auto-discovered running instances that are not yet registered
// as running instances by Sablier.
// By default, Sablier does not stop all already running instances. Meaning that you need to make an
// initial request in order to trigger the scaling to zero.
func StopAllUnregisteredInstances(ctx context.Context, provider providers.Provider, s store.Store, logger *slog.Logger) error {
	instances, err := provider.InstanceList(ctx, providers.InstanceListOptions{
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
		waitGroup.Go(stopFunc(ctx, name, provider, logger))
	}

	return waitGroup.Wait()
}

func stopFunc(ctx context.Context, name string, provider providers.Provider, logger *slog.Logger) func() error {
	return func() error {
		err := provider.Stop(ctx, name)
		if err != nil {
			logger.ErrorContext(ctx, "failed to stop instance", slog.String("instance", name), slog.Any("error", err))
			return err
		}
		logger.InfoContext(ctx, "stopped unregistered instance", slog.String("instance", name), slog.String("reason", "instance is enabled but not started by Sablier"))
		return nil
	}
}
