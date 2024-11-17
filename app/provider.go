package app

import (
	"context"
)

type EventAction string

const (
	// EventActionCreate describes when a workload has been created
	EventActionCreate EventAction = "create"

	// EventActionDestroy describes when a workload has been destroyed
	EventActionDestroy EventAction = "destroy"

	// EventActionReady describes when a workload is ready to handle traffic
	EventActionReady EventAction = "ready"

	// EventActionStart describes when a workload is started
	EventActionStart EventAction = "start"

	EventActionStop EventAction = "stop"
)

type Message struct {
	Name   string
	Group  string
	Action EventAction
}

type Provider interface {
	Start(ctx context.Context, name string, opts StartOptions) error
	Stop(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (bool, error)

	Events(ctx context.Context) (<-chan Message, <-chan error)
	SubscribeOnce(ctx context.Context, name string, action EventAction, wait chan<- error)
}
