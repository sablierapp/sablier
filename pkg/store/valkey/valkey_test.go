package valkey

import (
	"context"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/testcontainers/testcontainers-go"
	tcvalkey "github.com/testcontainers/testcontainers-go/modules/valkey"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func setupValKeyContainer(t *testing.T) valkey.Client {
	t.Helper()
	ctx := context.Background()
	c, err := tcvalkey.Run(ctx, "valkey/valkey:7.2.5",
		tcvalkey.WithLogLevel(tcvalkey.LogLevelDebug),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("6379/tcp")),
	)
	testcontainers.CleanupContainer(t, c)
	assert.NilError(t, err)

	uri, err := c.ConnectionString(ctx)
	assert.NilError(t, err)

	options, err := valkey.ParseURL(uri)
	assert.NilError(t, err)

	client, err := valkey.NewClient(options)
	assert.NilError(t, err)

	return client
}

func setupValKey(t *testing.T) *ValKey {
	t.Helper()
	client := setupValKeyContainer(t)
	vk, err := New(context.Background(), client)
	assert.NilError(t, err)
	return vk.(*ValKey)
}

func TestValKey(t *testing.T) {
	t.Parallel()
	t.Run("ValKeyErrNotFound", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		vk := setupValKey(t)

		_, err := vk.Get(ctx, "test")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("ValKeyPut", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		vk := setupValKey(t)

		err := vk.Put(ctx, instance.State{Name: "test"}, 1*time.Second)
		assert.NilError(t, err)

		i, err := vk.Get(ctx, "test")
		assert.NilError(t, err)
		assert.Equal(t, i.Name, "test")

		<-time.After(2 * time.Second)
		_, err = vk.Get(ctx, "test")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("ValKeyDelete", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		vk := setupValKey(t)

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
	t.Run("ValKeyOnExpire", func(t *testing.T) {
		t.Parallel()
		vk := setupValKey(t)

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
