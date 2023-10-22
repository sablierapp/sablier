package session

import (
	"context"

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
	// The Name of the targeted container, serivce, deployment
	// of which the state is being represented
	Name   string         `json:"name"`
	Status InstanceStatus `json:"status"`
}

type StartOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
}

func StartInstance(ctx context.Context, name string, opts StartOptions, p provider.Client) *promise.Promise[Instance] {
	return promise.New(func(resolve func(Instance), reject func(error)) {
		ready, err := p.Status(ctx, name)
		if err != nil {
			reject(err)
			return
		}

		if !ready {
			wait := make(chan error, 1)
			defer close(wait)
			p.SubscribeOnce(ctx, name, provider.EventActionStart, wait)
			startOpts := provider.StartOptions{DesiredReplicas: opts.DesiredReplicas}

			if err := p.Start(ctx, name, startOpts); err != nil {
				reject(err)
			}

			if err = <-wait; err != nil {
				reject(err)
			}
		}

		started := Instance{
			Name:   name,
			Status: InstanceRunning,
		}
		resolve(started)
	})
}
