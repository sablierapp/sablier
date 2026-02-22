package sablier

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package providertest -source=provider.go -destination=../provider/providertest/mock_provider.go *

type Provider interface {
	InstanceStart(ctx context.Context, name string) error
	InstanceStop(ctx context.Context, name string) error
	InstanceInspect(ctx context.Context, name string) (InstanceInfo, error)
	InstanceGroups(ctx context.Context) (map[string][]string, error)
	InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]InstanceConfiguration, error)

	NotifyInstanceStopped(ctx context.Context, instance chan<- string)
}
