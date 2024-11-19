package sablier_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/promise"
	pmock "github.com/sablierapp/sablier/pkg/provider/mock"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStartInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "myinstance"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 5 * time.Second,
		Timeout:            30 * time.Second,
		ExpiresAfter:       1 * time.Minute,
	}
	m := pmock.NewMockProvider(t)
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(nil).Once()
	s := sablier.NewSablier(ctx, m)

	p := s.StartInstance(name, opts)
	instance, err := p.Await(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "myinstance", instance.Name)
	assert.Equal(t, sablier.InstanceRunning, instance.Status)
}

func TestStartSamePromise(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "myinstance"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 5 * time.Second,
		Timeout:            30 * time.Second,
		ExpiresAfter:       1 * time.Minute,
	}
	m := pmock.NewMockProvider(t)
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(nil).Once()

	s := sablier.NewSablier(ctx, m)

	p1 := s.StartInstance(name, opts)
	p2 := s.StartInstance(name, opts)

	_, err := promise.All(context.Background(), p1, p2).Await(context.Background())
	assert.NoError(t, err)

	assert.Same(t, p1, p2)
}

func TestStartExpires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "myinstance"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 5 * time.Second,
		Timeout:            30 * time.Second,
		ExpiresAfter:       1 * time.Second,
	}
	m := pmock.NewMockProvider(t)
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(nil).Twice()
	m.EXPECT().Stop(mock.Anything, name).Return(nil).Once()

	s := sablier.NewSablier(ctx, m)

	p1 := s.StartInstance(name, opts)
	_, err := p1.Await(context.Background())
	assert.NoError(t, err)

	<-time.After(opts.ExpiresAfter * 2)

	p2 := s.StartInstance(name, opts)
	_, err = p2.Await(context.Background())
	assert.NoError(t, err)

	assert.NotSame(t, p1, p2)
}

func TestStartRefreshes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "myinstance"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 0,
		Timeout:            30 * time.Second,
		ExpiresAfter:       2 * time.Second,
	}
	m := pmock.NewMockProvider(t)
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).RunAndReturn(func(_ context.Context, _ string, _ sablier.StartOptions) error {
		<-time.After(1000 * time.Millisecond)
		return nil
	}).Once()

	s := sablier.NewSablier(ctx, m)

	// First call creates a new promise
	p1 := s.StartInstance(name, opts)

	<-time.After(500 * time.Millisecond)

	// Second call returns the pending promise
	p2 := s.StartInstance(name, opts)

	<-time.After(1500 * time.Millisecond)

	// Third call refreshes the duration on the already fulfilled promise
	p3 := s.StartInstance(name, opts)

	assert.Same(t, p1, p2, p3)

	_, err := promise.AllSettled(context.Background(), p1, p2, p3).Await(context.Background())
	assert.NoError(t, err)
}

func TestStartAgainOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "myinstance"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 5 * time.Second,
		Timeout:            30 * time.Second,
		ExpiresAfter:       5 * time.Second,
	}
	m := pmock.NewMockProvider(t)
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(fmt.Errorf("some error happened")).Once()
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(nil).Once()

	s := sablier.NewSablier(ctx, m)

	p1 := s.StartInstance(name, opts)
	_, err := p1.Await(context.Background())
	assert.Error(t, err)

	p2 := s.StartInstance(name, opts)
	_, err = p2.Await(context.Background())
	assert.NoError(t, err)

	assert.NotSame(t, p1, p2)
}

func TestStartInstanceError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "myinstance"
	opts := sablier.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 5 * time.Second,
		Timeout:            30 * time.Second,
		ExpiresAfter:       1 * time.Minute,
	}
	m := pmock.NewMockProvider(t)
	m.EXPECT().Start(mock.Anything, name, sablier.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	}).Return(errors.New("myinstance container not found")).Once()
	s := sablier.NewSablier(ctx, m)

	p := s.StartInstance(name, opts)
	_, err := p.Await(ctx)

	assert.Error(t, err)
}
