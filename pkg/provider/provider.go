package provider

import (
	"context"
	"github.com/sablierapp/sablier/app/types"

	"github.com/sablierapp/sablier/app/instance"
)

//go:generate go tool mockgen -package providertest -source=provider.go -destination=providertest/mock_provider.go *

type Provider interface {
	InstanceStart(ctx context.Context, name string) error
	InstanceStop(ctx context.Context, name string) error
	InstanceInspect(ctx context.Context, name string) (instance.State, error)
	InstanceGroups(ctx context.Context) (map[string][]string, error)
	InstanceList(ctx context.Context, options InstanceListOptions) ([]types.Instance, error)

	NotifyInstanceStopped(ctx context.Context, instance chan<- string)
}
