package provider

import (
	"context"
	"time"
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
	Name   string
	Group  string
	Action EventAction
}

type StartOptions struct {
	DesiredReplicas    uint32
	ConsiderReadyAfter time.Duration
}

type ListOptions struct {
	// All list all instances whatever their status (up or down)
	All bool
}

type Provider interface {
	Start(ctx context.Context, name string, opts StartOptions) error
	Stop(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (bool, error)
	List(ctx context.Context, opts ListOptions) ([]string, error)

	Events(ctx context.Context) (<-chan Message, <-chan error)
}
