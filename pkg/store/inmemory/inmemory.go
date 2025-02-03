package inmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/sablierapp/sablier/pkg/tinykv"
	"time"
)

var _ store.Store = (*InMemory)(nil)
var _ json.Marshaler = (*InMemory)(nil)
var _ json.Unmarshaler = (*InMemory)(nil)

func NewInMemory() store.Store {
	return &InMemory{
		kv: tinykv.New[instance.State](1*time.Second, nil),
	}
}

type InMemory struct {
	kv tinykv.KV[instance.State]
}

func (i InMemory) UnmarshalJSON(bytes []byte) error {
	return i.kv.UnmarshalJSON(bytes)
}

func (i InMemory) MarshalJSON() ([]byte, error) {
	return i.kv.MarshalJSON()
}

func (i InMemory) Get(_ context.Context, s string) (instance.State, error) {
	val, ok := i.kv.Get(s)
	if !ok {
		return instance.State{}, fmt.Errorf("key not found: %w", store.ErrKeyNotFound)
	}
	return val, nil
}

func (i InMemory) Put(_ context.Context, state instance.State, duration time.Duration) error {
	return i.kv.Put(state.Name, state, duration)
}

func (i InMemory) Delete(_ context.Context, s string) error {
	i.kv.Delete(s)
	return nil
}

func (i InMemory) OnExpire(_ context.Context, f func(string)) error {
	i.kv.SetOnExpire(func(k string, _ instance.State) {
		f(k)
	})
	return nil
}
