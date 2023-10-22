package mock

import (
	"context"

	"github.com/acouvreur/sablier/internal/provider"
)

// ProviderMock is a structure that allows to define the behavior of a Provider
type ProviderMock struct {
	StartFunc         func(ctx context.Context, name string, opts provider.StartOptions) error
	StopFunc          func(ctx context.Context, name string) error
	StatusFunc        func(ctx context.Context, name string) (bool, error)
	EventsFunc        func(ctx context.Context) (<-chan provider.Message, <-chan error)
	SubscribeOnceFunc func(ctx context.Context, name string, action provider.EventAction, wait chan<- error)
	DiscoverFunc      func(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error)
}

func (m *ProviderMock) Start(ctx context.Context, name string, opts provider.StartOptions) error {
	return m.StartFunc(ctx, name, opts)
}
func (m *ProviderMock) Stop(ctx context.Context, name string) error {
	return m.StopFunc(ctx, name)
}
func (m *ProviderMock) Status(ctx context.Context, name string) (bool, error) {
	return m.StatusFunc(ctx, name)
}
func (m *ProviderMock) Events(ctx context.Context) (<-chan provider.Message, <-chan error) {
	return m.EventsFunc(ctx)
}
func (m *ProviderMock) SubscribeOnce(ctx context.Context, name string, action provider.EventAction, wait chan<- error) {
	m.SubscribeOnceFunc(ctx, name, action, wait)
}

func (m *ProviderMock) Discover(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error) {
	return m.DiscoverFunc(ctx, opts)
}
