package sablier

import (
	"context"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/sablierapp/sablier/pkg/promise"
	"github.com/sablierapp/sablier/pkg/tinykv"
	"sync"
)

type PubSub interface {
	message.Publisher
	message.Subscriber
}

type Provider interface {
	Start(ctx context.Context, name string, opts StartOptions) error
	Stop(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (bool, error)

	Events(ctx context.Context) (<-chan Message, <-chan error)
}

type Sablier struct {
	provider    Provider
	promises    map[string]*promise.Promise[Instance]
	expirations tinykv.KV[string]

	pubsub PubSub

	lock sync.RWMutex
}
