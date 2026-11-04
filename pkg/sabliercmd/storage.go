package sabliercmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/configmap"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const serviceAccountNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// setupStorage creates the session store and restores any persisted state,
// from a file or a Kubernetes ConfigMap. The returned function must be called
// on shutdown to perform a final save.
func setupStorage(ctx context.Context, logger *slog.Logger, conf config.Config) (sablier.Store, func(), error) {
	if err := conf.Storage.IsValid(); err != nil {
		return nil, nil, err
	}
	if conf.Storage.Kubernetes.ConfigMap != "" {
		return setupConfigMapStorage(ctx, logger, conf.Storage.Kubernetes)
	}

	store := inmemory.NewInMemory()

	if conf.Storage.File != "" {
		loadFromFile(ctx, logger, conf.Storage.File, store)
	}

	save := func() {
		if conf.Storage.File != "" {
			saveToFile(ctx, logger, conf.Storage.File, store)
		}
	}

	return store, save, nil
}

// setupConfigMapStorage restores the sessions from the ConfigMap and starts
// writing them back as they change. A ConfigMap that cannot be read stops
// Sablier from starting: starting empty would overwrite the persisted sessions.
func setupConfigMapStorage(ctx context.Context, logger *slog.Logger, conf config.KubernetesStorage) (sablier.Store, func(), error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("storage.kubernetes.configmap: %w", err)
	}
	cli, err := k8s.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("storage.kubernetes.configmap: %w", err)
	}
	namespace := conf.Namespace
	if namespace == "" {
		if namespace, err = inClusterNamespace(); err != nil {
			return nil, nil, fmt.Errorf("storage.kubernetes.configmap: %w", err)
		}
	}

	store := configmap.New(cli.CoreV1().ConfigMaps(namespace), conf.ConfigMap, logger.With(slog.String("namespace", namespace)))
	if err := store.Load(ctx); err != nil {
		return nil, nil, fmt.Errorf("storage.kubernetes.configmap: %w", err)
	}
	go store.Run(ctx)

	save := func() {
		// ctx is already cancelled on shutdown.
		if err := store.Save(context.WithoutCancel(ctx)); err != nil {
			logger.ErrorContext(ctx, "cannot persist sessions on shutdown", slog.Any("error", err))
		}
	}
	return store, save, nil
}

// inClusterNamespace returns $POD_NAMESPACE, then the mounted service account namespace.
func inClusterNamespace() (string, error) {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns, nil
	}
	b, err := os.ReadFile(serviceAccountNamespaceFile)
	if err != nil {
		return "", fmt.Errorf("cannot determine the pod namespace, set POD_NAMESPACE: %w", err)
	}
	if ns := strings.TrimSpace(string(b)); ns != "" {
		return ns, nil
	}
	return "", errors.New("service account namespace file is empty")
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
