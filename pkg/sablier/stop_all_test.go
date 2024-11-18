package sablier_test

import (
	"context"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	pmock "github.com/sablierapp/sablier/pkg/provider/mock"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStopAllUnregistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := pmock.NewMockProvider(t)
	s := sablier.NewSablier(ctx, m)

	m.EXPECT().
		List(ctx, provider.ListOptions{All: false}).
		Return([]string{"instance1", "instance2"}, nil)
	m.EXPECT().Stop(ctx, "instance1").Return(nil).Once()
	m.EXPECT().Stop(ctx, "instance2").Return(nil).Once()
	err := s.StopAllUnregistered(ctx)
	assert.NoError(t, err)
}

func TestStopAllUnregisteredWithAlreadyRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "instance1"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 5 * time.Second,
		Timeout:            30 * time.Second,
		ExpiresAfter:       1 * time.Minute,
	}
	m := pmock.NewMockProvider(t)
	s := sablier.NewSablier(ctx, m)

	m.EXPECT().Start(mock.Anything, name, provider.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(nil).Once()
	p := s.StartInstance(name, opts)
	_, err := p.Await(ctx)
	assert.NoError(t, err)

	m.EXPECT().
		List(ctx, provider.ListOptions{All: false}).
		Return([]string{"instance1", "instance2"}, nil)
	m.EXPECT().Stop(ctx, "instance2").Return(nil).Once()
	err = s.StopAllUnregistered(ctx)
	assert.NoError(t, err)
}
