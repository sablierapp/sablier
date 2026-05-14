package sablier_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestGroupWatch_ContextDone(t *testing.T) {
	s, _, _ := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan struct{})
	go func() {
		s.GroupWatch(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("group watch did not stop on context cancellation")
	}
}

func TestGroupWatch_UpdatesGroupsOnTick(t *testing.T) {
	s, _, p := setupSablier(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	called := make(chan struct{}, 1)
	p.EXPECT().InstanceGroups(gomock.Any()).DoAndReturn(func(context.Context) (map[string][]string, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return map[string][]string{"g": {"a", "b"}}, nil
	}).AnyTimes()

	go s.GroupWatch(ctx)

	select {
	case <-called:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("group watch did not poll provider")
	}

	assert.DeepEqual(t, s.Groups(), map[string][]string{"g": {"a", "b"}})
}

func TestGroupWatch_ProviderErrorDoesNotUpdateGroups(t *testing.T) {
	s, _, p := setupSablier(t)
	s.SetGroups(map[string][]string{"existing": {"x"}})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	called := make(chan struct{}, 1)
	p.EXPECT().InstanceGroups(gomock.Any()).DoAndReturn(func(context.Context) (map[string][]string, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil, errors.New("provider down")
	}).AnyTimes()

	go s.GroupWatch(ctx)

	select {
	case <-called:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("group watch did not poll provider")
	}

	assert.DeepEqual(t, s.Groups(), map[string][]string{"existing": {"x"}})
}
