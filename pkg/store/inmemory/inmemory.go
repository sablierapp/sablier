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
		// Values are versioned session records; SessionRecord's unmarshaler
		// transparently upgrades snapshot files written before versioning.
		kv: tinykv.New[sablier.SessionRecord](1*time.Second, nil),
	}
}

type InMemory struct {
	kv tinykv.KV[sablier.SessionRecord]
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
	return val.Instance, nil
}

func (i InMemory) Put(_ context.Context, state sablier.InstanceInfo, duration time.Duration) error {
	return i.kv.Put(state.Name, sablier.NewSessionRecord(state), duration)
}

func (i InMemory) Delete(_ context.Context, s string) error {
	i.kv.Delete(s)
	return nil
}

func (i InMemory) Range(_ context.Context, f func(sablier.InstanceInfo, time.Time)) error {
	i.kv.Range(func(_ string, value sablier.SessionRecord, expiresAt time.Time) {
		f(value.Instance, expiresAt)
	})
	return nil
}

func (i InMemory) OnExpire(_ context.Context, f func(string)) error {
	i.kv.SetOnExpire(func(k string, _ sablier.SessionRecord) {
		f(k)
	})
	return nil
}
