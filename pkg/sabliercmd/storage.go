package sabliercmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
)

// setupStorage creates the session store, restores any persisted state from disk,
// and starts a periodic flush goroutine when a storage file is configured.
// The returned function must be called on shutdown to perform a final flush.
func setupStorage(ctx context.Context, logger *slog.Logger, conf config.Config) (sablier.Store, func()) {
	store := inmemory.NewInMemory()

	if conf.Storage.File != "" {
		loadFromFile(ctx, logger, conf.Storage.File, store)
	}

	save := func() {
		if conf.Storage.File != "" {
			saveToFile(ctx, logger, conf.Storage.File, store)
		}
	}

	return store, save
}

// loadFromFile restores persisted state into store from the given file path.
// A missing file is silently ignored. Any other error logs a warning and leaves
// the store empty so the server starts fresh.
func loadFromFile(ctx context.Context, logger *slog.Logger, file string, store sablier.Store) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		initializeFile(ctx, logger, file, store)
		return
	}
	if err != nil {
		logger.WarnContext(ctx, "failed to read state file, starting fresh",
			slog.String("file", file), slog.Any("error", err))
		return
	}

	imStore, ok := store.(*inmemory.InMemory)
	if !ok {
		return
	}
	if err := json.Unmarshal(data, imStore); err != nil {
		logger.WarnContext(ctx, "failed to restore state from file, starting fresh",
			slog.String("file", file), slog.Any("error", err))
		return
	}
	logger.InfoContext(ctx, "state restored from file", slog.String("file", file))
}

// initializeFile writes an empty store snapshot to file so the path exists for
// subsequent saves. A failure is non-fatal and is logged as a warning.
func initializeFile(ctx context.Context, logger *slog.Logger, file string, store sablier.Store) {
	imStore, ok := store.(*inmemory.InMemory)
	if !ok {
		return
	}
	data, err := json.Marshal(imStore)
	if err != nil {
		logger.WarnContext(ctx, "failed to initialize state file",
			slog.String("file", file), slog.Any("error", err))
		return
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		logger.WarnContext(ctx, "failed to initialize state file",
			slog.String("file", file), slog.Any("error", err))
	}
}

// saveToFile serializes the store and writes it to file with restricted permissions.
func saveToFile(ctx context.Context, logger *slog.Logger, file string, store sablier.Store) {
	imStore, ok := store.(*inmemory.InMemory)
	if !ok {
		return
	}
	data, err := json.Marshal(imStore)
	if err != nil {
		logger.ErrorContext(ctx, "failed to marshal state for persistence", slog.Any("error", err))
		return
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		logger.ErrorContext(ctx, "failed to save state to file",
			slog.String("file", file), slog.Any("error", err))
	}
}
