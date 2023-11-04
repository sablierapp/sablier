package session

import (
	"context"

	log "log/slog"

	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/pkg/promise"
)

type InstanceStatus string

const (
	InstanceStarting InstanceStatus = "starting"
	InstanceRunning  InstanceStatus = "running"
	InstanceError    InstanceStatus = "error"
)

// Instance holds the data representing an instance status
type Instance struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name   string         `json:"name"`
	Status InstanceStatus `json:"status"`
	Error  error          `json:"error"`
}

type StartOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
}

func StartInstance(name string, opts StartOptions, p provider.Client) *promise.Promise[Instance] {
	ctx := context.Background()
	pr := promise.New(func(resolve func(Instance), reject func(error)) {
		log.Info("starting instance", "instance", name)
		ready, err := p.Status(ctx, name)
		if err != nil {
			log.Info("error starting instance", "instance", name, "error", err)
			reject(err)
			return
		}

		if !ready {
			wait := make(chan error, 1)
			defer close(wait)
			p.SubscribeOnce(ctx, name, provider.EventActionStart, wait)
			startOpts := provider.StartOptions{DesiredReplicas: opts.DesiredReplicas}

			if err := p.Start(ctx, name, startOpts); err != nil {
				log.Info("error starting instance", "instance", name, "error", err)
				reject(err)
			}

			if err = <-wait; err != nil {
				log.Info("error starting instance", "instance", name, "error", err)
				reject(err)
			}
		}

		started := Instance{
			Name:   name,
			Status: InstanceRunning,
			Error:  nil,
		}
		log.Info("successfully started instance", "instance", name)
		resolve(started)
	})
	return pr
}
