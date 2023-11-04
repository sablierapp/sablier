package session_test

import (
	"context"
	"errors"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/provider/mock"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRequestRunningInstance(t *testing.T) {
	ctx := context.Background()
	m := &mock.ProviderMock{
		EventsFunc: func(ctx context.Context) (<-chan provider.Message, <-chan error) {
			return nil, nil
		},
		StatusFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}
	manager := session.NewManager(m, config.NewSessionsConfig())

	instances, _ := manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})

	assert.Equal(t, "myinstance", instances[0].Name)
	// An Instance which is ready will be "Starting" on the first request
	assert.Equal(t, session.InstanceStarting, instances[0].Status)
}

func TestRequestStartingInstance(t *testing.T) {
	ctx := context.Background()
	m := &mock.ProviderMock{
		EventsFunc: func(ctx context.Context) (<-chan provider.Message, <-chan error) {
			return nil, nil
		},
		StatusFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
		StartFunc: func(ctx context.Context, name string, opts provider.StartOptions) error {
			return nil
		},
		SubscribeOnceFunc: func(ctx context.Context, name string, action provider.EventAction, wait chan<- error) {
			wait <- nil
		},
	}
	manager := session.NewManager(m, config.NewSessionsConfig())

	instances, _ := manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceStarting, instances[0].Status)

	instances, _ = manager.RequestBlocking(ctx, []string{"myinstance"}, session.RequestBlockingOptions{})
	assert.Equal(t, session.InstanceRunning, instances[0].Status)

	instances, _ = manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceRunning, instances[0].Status)
}

func TestRequestErrorInstance(t *testing.T) {
	ctx := context.Background()
	ch := make(chan error, 1)
	m := &mock.ProviderMock{
		EventsFunc: func(ctx context.Context) (<-chan provider.Message, <-chan error) {
			return nil, nil
		},
		StatusFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
		StartFunc: func(ctx context.Context, name string, opts provider.StartOptions) error {
			return nil
		},
		SubscribeOnceFunc: func(ctx context.Context, name string, action provider.EventAction, wait chan<- error) {
			wait <- <-ch
		},
	}
	manager := session.NewManager(m, config.NewSessionsConfig())

	// The first request returns immediately before completion, so it's marked as starting
	ch <- errors.New("unexpected error please retry")
	instances, _ := manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceStarting, instances[0].Status)

	// We wait for the initial completion
	_, err := manager.RequestBlocking(ctx, []string{"myinstance"}, session.RequestBlockingOptions{})
	assert.Equal(t, "unexpected error please retry", err.Error())

	// The second request will be marked as error because the initial requested completed with error
	ch <- nil
	instances, _ = manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceError, instances[0].Status)

	// Then, the third call actually tells us that it's starting
	instances, _ = manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceStarting, instances[0].Status)

	// We wait for the second third completion
	instances, _ = manager.RequestBlocking(ctx, []string{"myinstance"}, session.RequestBlockingOptions{})
	assert.Equal(t, session.InstanceRunning, instances[0].Status)

	instances, _ = manager.RequestDynamic(ctx, []string{"myinstance"}, session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceRunning, instances[0].Status)
}
