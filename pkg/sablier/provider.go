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
	EventActionRemove EventAction = "remove"

	// EventActionStart describes when a workload is started and ready
	EventActionStart EventAction = "start"

	// EventActionStop describes when a workload is stopped
	EventActionStop EventAction = "stop"
)

type Message struct {
	Instance InstanceConfig
	Action   EventAction
}

type Provider interface {
	Start(ctx context.Context, name string, opts provider.StartOptions) error
	Stop(ctx context.Context, name string) error
	Info(ctx context.Context, name string) (InstanceInfo, error)

	List(ctx context.Context, opts provider.ListOptions) ([]InstanceConfig, error)

	Events(ctx context.Context) (<-chan Message, <-chan error)
}
