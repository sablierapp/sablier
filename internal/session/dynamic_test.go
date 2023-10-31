package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/provider/mock"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/stretchr/testify/assert"
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
	manager := session.NewSessionManager(m, config.NewSessionsConfig())

	instance, _ := manager.RequestDynamic(ctx, "myinstance", session.RequestDynamicOptions{})

	assert.Equal(t, "myinstance", instance.Name)
	// An Instance which is ready will be "Starting" on the first request
	assert.Equal(t, session.InstanceStarting, instance.Status)
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
	manager := session.NewSessionManager(m, config.NewSessionsConfig())

	instance, _ := manager.RequestDynamic(ctx, "myinstance", session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceStarting, instance.Status)

	instance, _ = manager.RequestBlocking(ctx, "myinstance", session.RequestBlockingOptions{
		Timeout: 10 * time.Second,
	})
	assert.Equal(t, session.InstanceRunning, instance.Status)

	instance, _ = manager.RequestDynamic(ctx, "myinstance", session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceRunning, instance.Status)
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
	manager := session.NewSessionManager(m, config.NewSessionsConfig())

	// The first request won't succeed
	ch <- errors.New("unexpected error please retry")
	instance, _ := manager.RequestDynamic(ctx, "myinstance", session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceStarting, instance.Status)

	// We wait for the initial completion
	_, err := manager.RequestBlocking(ctx, "myinstance", session.RequestBlockingOptions{
		Timeout: 10 * time.Second,
	})
	assert.Equal(t, "unexpected error please retry", err.Error())

	// But the second request succeeds
	ch <- nil
	instance, _ = manager.RequestDynamic(ctx, "myinstance", session.RequestDynamicOptions{})
	assert.Equal(t, session.InstanceStarting, instance.Status)

	// We wait for the second request completion
	instance, _ = manager.RequestBlocking(ctx, "myinstance", session.RequestBlockingOptions{
		Timeout: 10 * time.Second,
	})
	assert.Equal(t, session.InstanceRunning, instance.Status)
}
