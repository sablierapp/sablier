package sablier

import (
	"context"
	"time"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package storetest -source=store.go -destination=../store/storetest/mocks_store.go *

type Store interface {
	Get(context.Context, string) (InstanceInfo, error)
	Put(context.Context, InstanceInfo, time.Duration) error
	Delete(context.Context, string) error
	OnExpire(context.Context, func(string)) error
	// Range calls f for every non-expired session currently held by the store,
	// passing the instance info and its absolute expiration time. Iterating with
	// Range is read-only: it never renews a session's timeout, so it is safe to
	// use for observability (e.g. metrics) without extending sessions.
	Range(context.Context, func(InstanceInfo, time.Time)) error
}
