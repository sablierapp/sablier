package inmemory

import (
	"context"
	"encoding/json"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/sablierapp/sablier/pkg/tinykv"
	"time"
)

var _ sablier.Store = (*InMemory)(nil)
var _ json.Marshaler = (*InMemory)(nil)
var _ json.Unmarshaler = (*InMemory)(nil)

func NewInMemory() sablier.Store {
	return &InMemory{
		kv: tinykv.New[sablier.InstanceInfo](1*time.Second, nil),
	}
}

type InMemory struct {
	kv tinykv.KV[sablier.InstanceInfo]
}

func (i InMemory) UnmarshalJSON(bytes []byte) error {
	return i.kv.UnmarshalJSON(bytes)
}

func (i InMemory) MarshalJSON() ([]byte, error) {
	return i.kv.MarshalJSON()
}

func (i InMemory) Get(_ context.Context, s string) (sablier.InstanceInfo, error) {
	val, ok := i.kv.Get(s)
	if !ok {
		return sablier.InstanceInfo{}, store.ErrKeyNotFound
	}
	return val, nil
}

func (i InMemory) Put(_ context.Context, state sablier.InstanceInfo, duration time.Duration) error {
	return i.kv.Put(state.Name, state, duration)
}

func (i InMemory) Delete(_ context.Context, s string) error {
	i.kv.Delete(s)
	return nil
}

func (i InMemory) OnExpire(_ context.Context, f func(string)) error {
	i.kv.SetOnExpire(func(k string, _ sablier.InstanceInfo) {
		f(k)
	})
	return nil
}
