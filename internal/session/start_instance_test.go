package session_test

import (
	"context"
	"testing"

	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/provider/mock"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/stretchr/testify/assert"
)

func TestStartInstanceReady(t *testing.T) {
	ctx := context.Background()
	m := &mock.ProviderMock{StatusFunc: func(ctx context.Context, name string) (bool, error) {
		return true, nil
	}}

	p := session.StartInstance("myinstance", session.StartOptions{}, m)
	instance, _ := p.Await(ctx)

	assert.Equal(t, "myinstance", instance.Name)
	assert.Equal(t, session.InstanceRunning, instance.Status)
}

func TestStartInstanceStarting(t *testing.T) {
	ctx := context.Background()
	m := &mock.ProviderMock{
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

	p := session.StartInstance("myinstance", session.StartOptions{}, m)
	instance, _ := p.Await(ctx)

	assert.Equal(t, "myinstance", instance.Name)
	assert.Equal(t, session.InstanceRunning, instance.Status)
}
