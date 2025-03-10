package sablier

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"log/slog"
	"sync"
	"time"
)

//go:generate go tool mockgen -package sabliertest -source=sablier.go -destination=sabliertest/mocks_sablier.go *

type Sablier interface {
	RequestSession(ctx context.Context, names []string, duration time.Duration) (*SessionState, error)
	RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (*SessionState, error)
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*SessionState, error)

	RemoveInstance(ctx context.Context, name string) error
	SetGroups(groups map[string][]string)
	StopAllUnregisteredInstances(ctx context.Context) error
	GroupWatch(ctx context.Context)
}

type sablier struct {
	provider Provider
	sessions Store

	groupsMu sync.RWMutex
	groups   map[string][]string

	l *slog.Logger
}

func New(logger *slog.Logger, store Store, provider Provider) Sablier {
	return &sablier{
		provider: provider,
		sessions: store,
		groupsMu: sync.RWMutex{},
		groups:   map[string][]string{},
		l:        logger,
	}
}

func (s *sablier) SetGroups(groups map[string][]string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	if groups == nil {
		groups = map[string][]string{}
	}
	if diff := cmp.Diff(s.groups, groups); diff != "" {
		// TODO: Change this log for a friendly logging, groups rarely change, so we can put some effort on displaying what changed
		s.l.Info("set groups", slog.Any("old", s.groups), slog.Any("new", groups), slog.Any("diff", diff))
		s.groups = groups
	}
}

func (s *sablier) RemoveInstance(ctx context.Context, name string) error {
	return s.sessions.Delete(ctx, name)
}
