package providers

import (
	"context"
	"github.com/acouvreur/sablier/app/types"

	"github.com/acouvreur/sablier/app/instance"
)

type Provider interface {
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	GetState(ctx context.Context, name string) (instance.State, error)
	GetGroups(ctx context.Context) (map[string][]string, error)
	InstanceList(ctx context.Context, options InstanceListOptions) ([]types.Instance, error)

	NotifyInstanceStopped(ctx context.Context, instance chan<- string)
}
