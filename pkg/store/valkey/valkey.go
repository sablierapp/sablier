package valkey

import (
	"context"
	"encoding/json"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/valkey-io/valkey-go"
	"log/slog"
	"strings"
	"time"
)

var _ sablier.Store = (*ValKey)(nil)

type ValKey struct {
	Client valkey.Client
}

func New(ctx context.Context, client valkey.Client) (sablier.Store, error) {
	err := client.Do(ctx, client.B().Ping().Build()).Error()
	if err != nil {
		return nil, err
	}

	err = client.Do(ctx, client.B().ConfigSet().ParameterValue().
		ParameterValue("notify-keyspace-events", "KEx").
		Build()).Error()
	if err != nil {
		return nil, err
	}

	return &ValKey{Client: client}, nil
}

func (v *ValKey) Get(ctx context.Context, s string) (sablier.InstanceInfo, error) {
	b, err := v.Client.Do(ctx, v.Client.B().Get().Key(s).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return sablier.InstanceInfo{}, store.ErrKeyNotFound
	}
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	// Session entries are stored as versioned records; SessionRecord's
	// unmarshaler transparently upgrades pre-versioning payloads.
	var r sablier.SessionRecord
	err = json.Unmarshal(b, &r)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	return r.ToInstanceInfo(), nil
}

func (v *ValKey) Put(ctx context.Context, state sablier.InstanceInfo, duration time.Duration) error {
	value, err := json.Marshal(sablier.NewSessionRecord(state))
	if err != nil {
		return err
	}

	return v.Client.Do(ctx, v.Client.B().Set().Key(state.Name).Value(string(value)).Ex(duration).Build()).Error()
}

func (v *ValKey) Delete(ctx context.Context, s string) error {
	return v.Client.Do(ctx, v.Client.B().Del().Key(s).Build()).Error()
}

func (v *ValKey) Range(ctx context.Context, f func(sablier.InstanceInfo, time.Time)) error {
	// SCAN guarantees a full iteration but may return the same key several times
	// (e.g. during a keyspace rehash). Deduplicate so the caller never observes a
	// session twice, which would otherwise produce duplicate metric series.
	seen := make(map[string]struct{})
	var cursor uint64
	for {
		entry, err := v.Client.Do(ctx, v.Client.B().Scan().Cursor(cursor).Count(100).Build()).AsScanEntry()
		if err != nil {
			return err
		}

		for _, key := range entry.Elements {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			// PTTL and GET are read-only: they never reset the key's TTL, so
			// enumerating sessions this way does not renew them.
			pttl, err := v.Client.Do(ctx, v.Client.B().Pttl().Key(key).Build()).AsInt64()
			if err != nil {
				return err
			}
			// -2: the key vanished between SCAN and PTTL. -1: the key has no
			// expiry, so it is not a session. Skip both; only keys with a live
			// TTL represent an active session.
			if pttl <= 0 {
				continue
			}
			// Compute the absolute expiry from the TTL observed right now, before
			// the GET + unmarshal below. Deriving it after those calls would push
			// the reported expiry later than the true TTL under load.
			expiresAt := time.Now().Add(time.Duration(pttl) * time.Millisecond)

			b, err := v.Client.Do(ctx, v.Client.B().Get().Key(key).Build()).AsBytes()
			if valkey.IsValkeyNil(err) {
				continue
			}
			if err != nil {
				return err
			}

			// The store may share its keyspace with keys that are not Sablier
			// sessions. Skip anything that is not a valid session record, or
			// whose payload does not belong to this key, instead of aborting the
			// whole enumeration on a single foreign or corrupt key.
			var r sablier.SessionRecord
			if err = json.Unmarshal(b, &r); err != nil {
				continue
			}
			if r.Name != key {
				continue
			}

			f(r.ToInstanceInfo(), expiresAt)
		}

		cursor = entry.Cursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (v *ValKey) OnExpire(ctx context.Context, f func(string)) error {
	go func() {
		err := v.Client.Receive(ctx, v.Client.B().Psubscribe().Pattern("__key*__:*").Build(), func(msg valkey.PubSubMessage) {
			if msg.Message == "expired" {
				split := strings.Split(msg.Channel, ":")
				key := split[len(split)-1]
				f(key)
			}
		})
		if err != nil {
			slog.Error("error subscribing", slog.Any("error", err))
		}
	}()
	return nil
}
