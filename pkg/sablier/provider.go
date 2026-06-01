package sablier

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package providertest -source=provider.go -destination=../provider/providertest/mock_provider.go *

// InstanceEvent wraps an instance state change with the event type that triggered it.
type InstanceEvent struct {
	Type provider.InstanceEventType
	Info InstanceInfo
}

// InstanceEventStream is returned by InstanceEvents.
// Events carries typed instance lifecycle notifications.
// Err carries a terminal error when the stream cannot be recovered;
// after an error is sent both channels are closed.
type InstanceEventStream struct {
	Events <-chan InstanceEvent
	Err    <-chan error
}

type Provider interface {
	InstanceStart(ctx context.Context, name string) error
	InstanceStop(ctx context.Context, name string) error
	InstanceInspect(ctx context.Context, name string) (InstanceInfo, error)
	InstanceGroups(ctx context.Context) (map[string][]string, error)
	InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]InstanceConfiguration, error)

	// InstanceDependencies returns the transitive dependencies of name in
	// topological order: each dependency is listed before any instance that
	// depends on it. Providers that do not support dependency ordering return
	// nil, nil.
	InstanceDependencies(ctx context.Context, name string) ([]InstanceDependency, error)

	InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) InstanceEventStream
}
