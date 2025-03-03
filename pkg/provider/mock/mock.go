package mock

import (
	"context"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/stretchr/testify/mock"
)

// Interface guard
var _ provider.Provider = (*ProviderMock)(nil)

// ProviderMock is a structure that allows to define the behavior of a Provider
type ProviderMock struct {
	mock.Mock
}

func (m *ProviderMock) InstanceStart(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *ProviderMock) InstanceStop(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *ProviderMock) InstanceInspect(ctx context.Context, name string) (instance.State, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(instance.State), args.Error(1)
}
func (m *ProviderMock) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string][]string), args.Error(1)
}
func (m *ProviderMock) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]types.Instance, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Instance), args.Error(1)
}

func (m *ProviderMock) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	m.Called(ctx, instance)
}
