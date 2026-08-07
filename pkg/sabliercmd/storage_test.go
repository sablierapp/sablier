package sabliercmd

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	storemod "github.com/sablierapp/sablier/pkg/store"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

// storeJSON builds a minimal serialised store entry compatible with tinykv's wire format.
func storeJSON(name string, expiresAt time.Time) []byte {
	b, _ := json.Marshal(map[string]any{
		name: map[string]any{
			"value": map[string]any{
				"name":            name,
				"currentReplicas": 1,
				"desiredReplicas": 1,
				"status":          "ready",
			},
			"expiresAt": expiresAt.UTC().Format(time.RFC3339),
		},
	})
	return b
}

func TestLoadFromFile_NotFound_CreatesFile(t *testing.T) {
	ctx := context.Background()
	store := inmemory.NewInMemory()

	path := filepath.Join(t.TempDir(), "state.json")
	loadFromFile(ctx, slog.Default(), path, store)

	// Store must be empty.
	_, err := store.Get(ctx, "anything")
	assert.ErrorIs(t, err, storemod.ErrKeyNotFound)

	// File must have been created.
	_, err = os.Stat(path)
	assert.NilError(t, err)
}

func TestLoadFromFile_NotFound_CannotCreate(t *testing.T) {
	ctx := context.Background()
	store := inmemory.NewInMemory()

	// Parent directory does not exist, so the write will fail.
	path := filepath.Join(t.TempDir(), "nodir", "state.json")

	// Must not panic; warning is logged internally.
	loadFromFile(ctx, slog.Default(), path, store)

	_, err := store.Get(ctx, "anything")
	assert.ErrorIs(t, err, storemod.ErrKeyNotFound)
}

func TestLoadFromFile_ValidEntry(t *testing.T) {
	ctx := context.Background()
	store := inmemory.NewInMemory()

	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, storeJSON("myservice", time.Now().Add(time.Hour)), 0o600))

	loadFromFile(ctx, slog.Default(), path, store)

	info, err := store.Get(ctx, "myservice")
	assert.NilError(t, err)
	assert.Equal(t, info.Name, "myservice")
}

func TestLoadFromFile_ExpiredEntry(t *testing.T) {
	ctx := context.Background()
	store := inmemory.NewInMemory()

	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, storeJSON("oldservice", time.Now().Add(-time.Hour)), 0o600))

	loadFromFile(ctx, slog.Default(), path, store)

	_, err := store.Get(ctx, "oldservice")
	assert.ErrorIs(t, err, storemod.ErrKeyNotFound)
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	store := inmemory.NewInMemory()

	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte("not valid json"), 0o600))

	// Must not panic; store stays empty.
	loadFromFile(ctx, slog.Default(), path, store)

	_, err := store.Get(ctx, "anything")
	assert.ErrorIs(t, err, storemod.ErrKeyNotFound)
}

func TestSaveToFile(t *testing.T) {
	ctx := context.Background()
	store := inmemory.NewInMemory()

	require.NoError(t, store.Put(ctx, sablier.InstanceInfo{
		Name: "myservice", Status: sablier.InstanceStatusReady,
		CurrentReplicas: 1, DesiredReplicas: 1,
	}, time.Hour))

	path := filepath.Join(t.TempDir(), "state.json")
	saveToFile(ctx, slog.Default(), path, store)

	// Round-trip: load the saved file into a fresh store and verify.
	fresh := inmemory.NewInMemory()
	loadFromFile(ctx, slog.Default(), path, fresh)

	info, err := fresh.Get(ctx, "myservice")
	assert.NilError(t, err)
	assert.Equal(t, info.Name, "myservice")
}

func TestSetupStorage_LoadOnStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, storeJSON("startup-service", time.Now().Add(time.Hour)), 0o600))

	conf := config.NewConfig()
	conf.Storage.File = path
	conf.Sessions.ExpirationInterval = time.Hour // slow periodic save, irrelevant here

	store, _ := setupStorage(ctx, slog.Default(), conf)

	info, err := store.Get(ctx, "startup-service")
	assert.NilError(t, err)
	assert.Equal(t, info.Name, "startup-service")
}

func TestSetupStorage_SaveOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(t.TempDir(), "state.json")

	conf := config.NewConfig()
	conf.Storage.File = path
	conf.Sessions.ExpirationInterval = time.Hour // slow periodic save, irrelevant here

	store, save := setupStorage(ctx, slog.Default(), conf)

	require.NoError(t, store.Put(ctx, sablier.InstanceInfo{
		Name: "shutdown-service", Status: sablier.InstanceStatusReady,
	}, time.Hour))

	// Simulate shutdown flush.
	save()

	// Verify the entry survives a round-trip through the file.
	fresh := inmemory.NewInMemory()
	loadFromFile(ctx, slog.Default(), path, fresh)

	info, err := fresh.Get(ctx, "shutdown-service")
	assert.NilError(t, err)
	assert.Equal(t, info.Name, "shutdown-service")
}
