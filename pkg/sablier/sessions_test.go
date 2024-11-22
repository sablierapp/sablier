package sablier_test

import (
	"context"
	"github.com/sablierapp/sablier/pkg/provider"
	pmock "github.com/sablierapp/sablier/pkg/provider/mock"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func TestStartSessionByNamesWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	names := []string{"instance1", "instance2", "instance3"}
	opts := sablier.StartSessionOptions{
		Wait: true,
		StartOptions: sablier.StartOptions{
			DesiredReplicas:    1,
			ConsiderReadyAfter: 5 * time.Second,
			Timeout:            30 * time.Second,
			ExpiresAfter:       1 * time.Minute,
		},
	}
	m := pmock.NewMockProvider(t)
	for _, name := range names {
		m.EXPECT().Start(mock.Anything, name, provider.StartOptions{
			DesiredReplicas:    opts.DesiredReplicas,
			ConsiderReadyAfter: opts.ConsiderReadyAfter,
		}).Return(nil).Once()
	}

	s := sablier.NewSablier(ctx, m)

	session, err := s.StartSessionByNames(ctx, names, opts)

	assert.NoError(t, err)
	assert.Equal(t, sablier.SessionStatusReady, session.Status)
}

func TestStartSessionByNames(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	names := []string{"instance1", "instance2", "instance3"}
	opts := sablier.StartSessionOptions{
		Wait: false,
		StartOptions: sablier.StartOptions{
			DesiredReplicas:    1,
			ConsiderReadyAfter: 5 * time.Second,
			Timeout:            30 * time.Second,
			ExpiresAfter:       1 * time.Minute,
		},
	}
	m := pmock.NewMockProvider(t)
	for _, name := range names {
		m.EXPECT().Start(mock.Anything, name, provider.StartOptions{
			DesiredReplicas:    opts.DesiredReplicas,
			ConsiderReadyAfter: opts.ConsiderReadyAfter,
		}).RunAndReturn(func(_ context.Context, _ string, _ provider.StartOptions) error {
			<-time.After(2 * time.Second)
			return nil
		}).Once()
		m.EXPECT().Info(mock.Anything, name).Return(sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStarting,
		}, nil).Once()
	}

	s := sablier.NewSablier(ctx, m)

	session, err := s.StartSessionByNames(ctx, names, opts)
	<-time.After(100 * time.Millisecond)
	assert.NoError(t, err)

	assert.Equal(t, sablier.SessionStatusNotReady, session.Status)
}
