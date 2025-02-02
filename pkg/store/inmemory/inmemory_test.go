package inmemory

import (
	"context"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/pkg/store"
	"gotest.tools/v3/assert"
	"testing"
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
		ctx := context.Background()
		vk := NewInMemory()

		err := vk.Put(ctx, instance.State{Name: "test"}, 1*time.Second)
		assert.NilError(t, err)

		i, err := vk.Get(ctx, "test")
		assert.NilError(t, err)
		assert.Equal(t, i.Name, "test")

		<-time.After(2 * time.Second)
		_, err = vk.Get(ctx, "test")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("InMemoryDelete", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		vk := NewInMemory()

		err := vk.Put(ctx, instance.State{Name: "test"}, 30*time.Second)
		assert.NilError(t, err)

		i, err := vk.Get(ctx, "test")
		assert.NilError(t, err)
		assert.Equal(t, i.Name, "test")

		err = vk.Delete(ctx, "test")
		assert.NilError(t, err)

		_, err = vk.Get(ctx, "test")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("InMemoryOnExpire", func(t *testing.T) {
		t.Parallel()
		vk := NewInMemory()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		expirations := make(chan string)
		err := vk.OnExpire(ctx, func(key string) {
			expirations <- key
		})
		assert.NilError(t, err)

		err = vk.Put(ctx, instance.State{Name: "test"}, 1*time.Second)
		assert.NilError(t, err)
		expired := <-expirations
		assert.Equal(t, expired, "test")
	})
}
