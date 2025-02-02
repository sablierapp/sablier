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
	Get(context.Context, string) (instance.State, error)
	Put(context.Context, instance.State, time.Duration) error
	Delete(context.Context, string) error
	OnExpire(context.Context, func(string)) error
}
