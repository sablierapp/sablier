package session_test

import (
	"context"
	"fmt"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/provider/mock"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRequestBlockingRunningInstance(t *testing.T) {
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

	instances, err := manager.RequestBlocking(ctx, []string{"myinstance"}, session.RequestBlockingOptions{})

	if err != nil {
		t.Error(err)
		t.Fail()
	}

	assert.Equal(t, "myinstance", instances[0].Name)
	assert.Equal(t, session.InstanceRunning, instances[0].Status)
}

func TestRequestBlockingStartingInstance(t *testing.T) {
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

	instances, _ := manager.RequestBlocking(ctx, []string{"myinstance"}, session.RequestBlockingOptions{})
	assert.Equal(t, session.InstanceRunning, instances[0].Status)
}

func TestRequestBlockingErrorInstance(t *testing.T) {
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
			wait <- fmt.Errorf("%s does not exist", name)
		},
	}
	manager := session.NewManager(m, config.NewSessionsConfig())

	_, err := manager.RequestBlocking(ctx, []string{"myinstance"}, session.RequestBlockingOptions{})

	assert.Equal(t, "myinstance does not exist", err.Error())
}
