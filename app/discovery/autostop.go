package discovery

import (
	"context"
	"github.com/sablierapp/sablier/app/providers"
	"github.com/sablierapp/sablier/pkg/arrays"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// StopAllUnregisteredInstances stops all auto-discovered running instances that are not yet registered
// as running instances by Sablier.
// By default, Sablier does not stop all already running instances. Meaning that you need to make an
// initial request in order to trigger the scaling to zero.
func StopAllUnregisteredInstances(ctx context.Context, provider providers.Provider, registered []string) error {
	log.Info("Stopping all unregistered running instances")

	log.Trace("Retrieving all registered instances")
	instances, err := provider.List(ctx)
	if err != nil {
		return err
	}

	log.Tracef("Found %v instances", len(instances))
	names := make([]string, 0, len(instances))
	for _, instance := range instances {
		names = append(names, instance.Name)
	}

	unregistered := arrays.RemoveElements(names, registered)
	log.Tracef("Found %v unregistered instances ", len(instances))

	waitGroup := errgroup.Group{}

	// Previously, the variables declared by a “for” loop were created once and updated by each iteration.
	// In Go 1.22, each iteration of the loop creates new variables, to avoid accidental sharing bugs.
	// The transition support tooling described in the proposal continues to work in the same way it did in Go 1.21.
	for _, name := range unregistered {
		waitGroup.Go(stopFunc(ctx, name, provider))
	}

	return waitGroup.Wait()
}

func stopFunc(ctx context.Context, name string, provider providers.Provider) func() error {
	return func() error {
		log.Tracef("Stopping %v...", name)
		err := provider.Stop(ctx, name)
		if err != nil {
			log.Errorf("Could not stop %v: %v", name, err)
			return err
		}
		log.Tracef("Successfully stopped %v", name)
		return nil
	}
}
