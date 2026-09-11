package inmemory

import (
	"context"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"gotest.tools/v3/assert"
	"testing"
	"testing/synctest"
	"time"
)

func TestInMemory(t *testing.T) {
	t.Parallel()
	t.Run("InMemoryErrNotFound", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		vk := NewInMemory()

		_, err := vk.Get(ctx, "test")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("InMemoryPut", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := context.Background()
			vk := newInMemory(t)

			err := vk.Put(ctx, sablier.InstanceInfo{Name: "test"}, 1*time.Second)
			assert.NilError(t, err)

			i, err := vk.Get(ctx, "test")
			assert.NilError(t, err)
			assert.Equal(t, i.Name, "test")

			synctest.Sleep(2 * time.Second)
			_, err = vk.Get(ctx, "test")
			assert.ErrorIs(t, err, store.ErrKeyNotFound)
		})
	})
	t.Run("InMemoryDelete", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		vk := NewInMemory()

		err := vk.Put(ctx, sablier.InstanceInfo{Name: "test"}, 30*time.Second)
		assert.NilError(t, err)

		i, err := vk.Get(ctx, "test")
		assert.NilError(t, err)
		assert.Equal(t, i.Name, "test")

		err = vk.Delete(ctx, "test")
		assert.NilError(t, err)

		_, err = vk.Get(ctx, "test")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("InMemoryRange", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := context.Background()
			vk := newInMemory(t)

			err := vk.Put(ctx, sablier.InstanceInfo{Name: "live", Groups: []string{"demo"}}, 30*time.Second)
			assert.NilError(t, err)
			err = vk.Put(ctx, sablier.InstanceInfo{Name: "expired"}, 10*time.Millisecond)
			assert.NilError(t, err)

			synctest.Sleep(30 * time.Millisecond)

			got := make(map[string]time.Time)
			err = vk.Range(ctx, func(info sablier.InstanceInfo, expiresAt time.Time) {
				got[info.Name] = expiresAt
			})
			assert.NilError(t, err)

			// Only the live session is enumerated; the expired one is skipped.
			assert.Equal(t, len(got), 1)
			exp, ok := got["live"]
			assert.Assert(t, ok)
			assert.Assert(t, exp.After(time.Now()))
		})
	})
	t.Run("InMemoryOnExpire", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			vk := newInMemory(t)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			expirations := make(chan string)
			err := vk.OnExpire(ctx, func(key string) {
				expirations <- key
			})
			assert.NilError(t, err)

			err = vk.Put(ctx, sablier.InstanceInfo{Name: "test"}, 1*time.Second)
			assert.NilError(t, err)
			expired := <-expirations
			assert.Equal(t, expired, "test")
		})
	})
}

// newInMemory stops the expiry goroutine at cleanup, because synctest.Test fails
// when a goroutine stays blocked. See https://pkg.go.dev/testing/synctest#Test
func newInMemory(t *testing.T) sablier.Store {
	s := NewInMemory()
	t.Cleanup(s.(*InMemory).kv.Stop)
	return s
}
