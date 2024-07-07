package mock

import (
	"context"
	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/app/types"
	"github.com/stretchr/testify/mock"
)

// Interface guard
var _ providers.Provider = (*ProviderMock)(nil)

// ProviderMock is a structure that allows to define the behavior of a Provider
type ProviderMock struct {
	mock.Mock
}

func (m *ProviderMock) Start(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *ProviderMock) Stop(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *ProviderMock) GetState(ctx context.Context, name string) (instance.State, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(instance.State), args.Error(1)
}
func (m *ProviderMock) GetGroups(ctx context.Context) (map[string][]string, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string][]string), args.Error(1)
}
func (m *ProviderMock) InstanceList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Instance), args.Error(1)
}

func (m *ProviderMock) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	m.Called(ctx, instance)
}
