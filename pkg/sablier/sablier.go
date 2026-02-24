package sablier

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/google/go-cmp/cmp"
	"github.com/sablierapp/sablier/pkg/provider"

	"github.com/nats-io/nats.go"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package providertest -source=provider.go -destination=../provider/providertest/mock_provider.go *

type Provider interface {
	InstanceStart(ctx context.Context, name string) error
	InstanceStop(ctx context.Context, name string) error
	InstanceInspect(ctx context.Context, name string) (InstanceInfo, error)
	InstanceGroups(ctx context.Context) (map[string][]string, error)
	InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]InstanceConfiguration, error)

	NotifyInstanceStopped(ctx context.Context, instance chan<- string)
}

//go:generate go tool -modfile=../../tools.mod mockgen -package storetest -source=store.go -destination=../store/storetest/mocks_store.go *

type Store interface {
	Get(context.Context, string) (InstanceInfo, error)
	Put(context.Context, InstanceInfo, time.Duration) error
	Delete(context.Context, string) error
	OnExpire(context.Context, func(string)) error
}

type Sablier struct {
	provider     Provider
	sessions     Store
	singleflight *singleflight.Group

	groupsMu sync.RWMutex
	groups   map[string][]string

	nc *nats.Conn

	// BlockingRefreshFrequency is the frequency at which the instances are checked
	// against the provider. Defaults to 5 seconds.
	BlockingRefreshFrequency time.Duration

	l *slog.Logger
}

func New(logger *slog.Logger, store Store, provider Provider, nc *nats.Conn) *Sablier {

	// Subscribe to the "updates" subject with a callback function
	// The callback receives messages as they are published
	_, err := nc.Subscribe("updates", func(m *nats.Msg) {
		fmt.Printf("Received message on subject %s: %s\n", m.Subject, string(m.Data))
	})
	if err != nil {
		logger.Error("failed to subscribe to updates subject", slog.Any("error", err))
	}

	return &Sablier{
		provider:                 provider,
		sessions:                 store,
		groupsMu:                 sync.RWMutex{},
		groups:                   map[string][]string{},
		l:                        logger,
		BlockingRefreshFrequency: 5 * time.Second,
		singleflight:             new(singleflight.Group),
		nc:                       nc,
	}
}

func (s *Sablier) SetGroups(groups map[string][]string) {
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

func (s *Sablier) RemoveInstance(ctx context.Context, name string) error {
	return s.sessions.Delete(ctx, name)
}
