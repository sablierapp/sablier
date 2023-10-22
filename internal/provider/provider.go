package provider

import (
	"context"
)

type StartOptions struct {
	DesiredReplicas uint32
}

type EventAction string

const (
	EventActionCreate  EventAction = "create"
	EventActionDestroy EventAction = "destroy"
	EventActionStart   EventAction = "start"
	EventActionStop    EventAction = "stop"
)

type Message struct {
	Name   string
	Action EventAction
}

type Client interface {
	Start(ctx context.Context, name string, opts StartOptions) error
	Stop(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (bool, error)
	Discover(ctx context.Context, opts DiscoveryOptions) ([]Discovered, error)

	Events(ctx context.Context) (<-chan Message, <-chan error)
	SubscribeOnce(ctx context.Context, name string, action EventAction, wait chan<- error)
}
