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
//
// Events carries typed instance lifecycle notifications. Streams are expected
// to survive transient failures by reconnecting internally (the Docker-API
// based providers redial with capped backoff, indefinitely) rather than dying
// on the first hiccup; consumers pair the stream with periodic reconciliation
// for anything missed while disconnected.
//
// Err carries a terminal error only when the provider concludes the stream
// cannot be recovered; most providers never emit one. After a terminal error
// both channels are closed. On context cancellation both channels are closed
// without an error.
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

	// InstanceDependencies returns the direct dependencies of name (the instances
	// it must wait for before starting), each with the condition it must reach.
	// Sablier core walks these hints transitively, detects cycles, and orders the
	// starts; providers only report one instance's immediate dependencies.
	// Providers that do not support dependencies return nil, nil.
	InstanceDependencies(ctx context.Context, name string) ([]InstanceDependency, error)

	InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) InstanceEventStream
}
