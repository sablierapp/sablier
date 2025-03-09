package valkey

import (
	"context"
	"github.com/sablierapp/sablier/pkg/sablier"
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
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	vk := setupValKey(t)

	t.Parallel()
	t.Run("ValKeyErrNotFound", func(t *testing.T) {
		t.Parallel()
		_, err := vk.Get(ctx, "ValKeyErrNotFound")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("ValKeyPut", func(t *testing.T) {
		t.Parallel()

		err := vk.Put(ctx, sablier.InstanceInfo{Name: "ValKeyPut"}, 30*time.Second)
		assert.NilError(t, err)

		i, err := vk.Get(ctx, "ValKeyPut")
		assert.NilError(t, err)
		assert.Equal(t, i.Name, "ValKeyPut")

		<-time.After(2 * time.Second)
		_, err = vk.Get(ctx, "ValKeyPut")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("ValKeyDelete", func(t *testing.T) {
		t.Parallel()

		err := vk.Put(ctx, sablier.InstanceInfo{Name: "ValKeyDelete"}, 30*time.Second)
		assert.NilError(t, err)

		i, err := vk.Get(ctx, "ValKeyDelete")
		assert.NilError(t, err)
		assert.Equal(t, i.Name, "ValKeyDelete")

		err = vk.Delete(ctx, "ValKeyDelete")
		assert.NilError(t, err)

		_, err = vk.Get(ctx, "ValKeyDelete")
		assert.ErrorIs(t, err, store.ErrKeyNotFound)
	})
	t.Run("ValKeyOnExpire", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		expirations := make(chan string)
		err := vk.OnExpire(ctx, func(key string) {
			expirations <- key
		})
		assert.NilError(t, err)

		err = vk.Put(ctx, sablier.InstanceInfo{Name: "ValKeyOnExpire"}, 1*time.Second)
		assert.NilError(t, err)
		expired := <-expirations
		assert.Equal(t, expired, "ValKeyOnExpire")
	})
}
