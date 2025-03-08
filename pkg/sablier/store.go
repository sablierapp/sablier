package sablier

import (
	"context"
	"time"
)

//go:generate go tool mockgen -package storetest -source=store.go -destination=../store/storetest/mocks_store.go *

type Store interface {
	Get(context.Context, string) (InstanceInfo, error)
	Put(context.Context, InstanceInfo, time.Duration) error
	Delete(context.Context, string) error
	OnExpire(context.Context, func(string)) error
}
