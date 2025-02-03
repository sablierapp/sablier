package store

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/app/instance"
	"time"
)

var ErrKeyNotFound = errors.New("key not found")

//go:generate mockgen -package storetest -source=store.go -destination=storetest/mocks_store.go *

type Store interface {
	Get(ctx context.Context, key string) (instance.State, error)
	Put(ctx context.Context, state instance.State, duration time.Duration) error
	Delete(ctx context.Context, key string) error
	OnExpire(ctx context.Context, callback func(string)) error
}
