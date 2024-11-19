package sablier

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
)

type EventAction string

const (
	// EventActionCreate describes when a workload has been created
	EventActionCreate EventAction = "create"

	// EventActionRemove describes when a workload has been destroyed
	EventActionRemove EventAction = "destroy"

	// EventActionReady describes when a workload is ready to handle traffic
	EventActionReady EventAction = "ready"

	// EventActionStart describes when a workload is started but not necessarily ready

	EventActionStart EventAction = "start"

	// EventActionStop describes when a workload is stopped
	EventActionStop EventAction = "stop"
)

type Message struct {
	Instance Instance
	Action   EventAction
}

type Provider interface {
	Start(ctx context.Context, name string, opts provider.StartOptions) error
	Stop(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (bool, error)

	List(ctx context.Context, opts provider.ListOptions) ([]Instance, error)

	Events(ctx context.Context) (<-chan Message, <-chan error)
}
