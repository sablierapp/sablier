package sablier

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package providertest -source=provider.go -destination=../provider/providertest/mock_provider.go *

// InstanceEventStream is returned by InstanceEvents.
// Events carries instance state change notifications.
// Err carries a terminal error when the stream cannot be recovered;
// after an error is sent both channels are closed.
type InstanceEventStream struct {
	Events <-chan InstanceInfo
	Err    <-chan error
}

type Provider interface {
	InstanceStart(ctx context.Context, name string) error
	InstanceStop(ctx context.Context, name string) error
	InstanceInspect(ctx context.Context, name string) (InstanceInfo, error)
	InstanceGroups(ctx context.Context) (map[string][]string, error)
	InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]InstanceConfiguration, error)

	InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) InstanceEventStream
}
