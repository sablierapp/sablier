package app

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/http/routes"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/app/storage"
	"github.com/sablierapp/sablier/app/theme"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/internal/server"
	"github.com/sablierapp/sablier/version"
)

func Start(ctx context.Context, conf config.Config) error {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := setupLogger(conf.Logging)

	logger.Info("running Sablier version " + version.Info())

	provider, err := NewProvider(ctx, logger, conf.Provider)
	if err != nil {
		return err
	}

	store := inmemory.NewInMemory()
	err = store.OnExpire(ctx, onSessionExpires(ctx, provider, logger))
	if err != nil {
		return err
	}

	sessionsManager := sessions.NewSessionsManager(logger, store, provider)

	if conf.Storage.File != "" {
		storage, err := storage.NewFileStorage(conf.Storage, logger)
		if err != nil {
			return err
		}

		defer saveSessions(storage, sessionsManager, logger)
		loadSessions(storage, sessionsManager, logger)
	}

	groups, err := provider.GetGroups(ctx)
	if err != nil {
		logger.WarnContext(ctx, "initial group scan failed", slog.Any("reason", err))
	} else {
		sessionsManager.SetGroups(groups)
	}

	updateGroups := make(chan map[string][]string)
	go WatchGroups(ctx, provider, 2*time.Second, updateGroups, logger)
	go func() {
		for groups := range updateGroups {
			sessionsManager.SetGroups(groups)
		}
	}()

	instanceStopped := make(chan string)
	go provider.NotifyInstanceStopped(ctx, instanceStopped)
	go func() {
		for stopped := range instanceStopped {
			err := sessionsManager.RemoveInstance(stopped)
			if err != nil {
				logger.Warn("could not remove instance", slog.Any("error", err))
			}
		}
	}()

	if conf.Provider.AutoStopOnStartup {
		err := discovery.StopAllUnregisteredInstances(ctx, provider, store, logger)
		if err != nil {
			logger.ErrorContext(ctx, "unable to stop unregistered instances", slog.Any("reason", err))
		}
	}

	var t *theme.Themes

	if conf.Strategy.Dynamic.CustomThemesPath != "" {
		logger.DebugContext(ctx, "loading themes from custom theme path", slog.String("path", conf.Strategy.Dynamic.CustomThemesPath))
		custom := os.DirFS(conf.Strategy.Dynamic.CustomThemesPath)
		t, err = theme.NewWithCustomThemes(custom, logger)
		if err != nil {
			return err
		}
	} else {
		logger.DebugContext(ctx, "loading themes without custom theme path", slog.String("reason", "--strategy.dynamic.custom-themes-path is empty"))
		t, err = theme.New(logger)
		if err != nil {
			return err
		}
	}

	strategy := &routes.ServeStrategy{
		Theme:           t,
		SessionsManager: sessionsManager,
		StrategyConfig:  conf.Strategy,
		SessionsConfig:  conf.Sessions,
	}

	go server.Start(ctx, logger, conf.Server, strategy)

	// Listen for the interrupt signal.
	<-ctx.Done()

	stop()
	logger.InfoContext(ctx, "shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.InfoContext(ctx, "Server exiting")

	return nil
}

func onSessionExpires(ctx context.Context, provider provider.Provider, logger *slog.Logger) func(key string) {
	return func(_key string) {
		go func(key string) {
			logger.InfoContext(ctx, "instance expired", slog.String("instance", key))
			err := provider.Stop(ctx, key)
			if err != nil {
				logger.ErrorContext(ctx, "instance expired could not be stopped from provider", slog.String("instance", key), slog.Any("error", err))
			}
		}(_key)
	}
}

func loadSessions(storage storage.Storage, sessions sessions.Manager, logger *slog.Logger) {
	logger.Info("loading sessions from storage")
	reader, err := storage.Reader()
	if err != nil {
		logger.Error("error loading sessions from storage", slog.Any("reason", err))
	}
	err = sessions.LoadSessions(reader)
	if err != nil {
		logger.Error("error loading sessions into Sablier", slog.Any("reason", err))
	}
}

func saveSessions(storage storage.Storage, sessions sessions.Manager, logger *slog.Logger) {
	logger.Info("writing sessions to storage")
	writer, err := storage.Writer()
	if err != nil {
		logger.Error("error saving sessions to storage", slog.Any("reason", err))
		return
	}
	err = sessions.SaveSessions(writer)
	if err != nil {
		logger.Error("error saving sessions from Sablier", slog.Any("reason", err))
	}
}

func NewProvider(ctx context.Context, logger *slog.Logger, config config.Provider) (provider.Provider, error) {
	if err := config.IsValid(); err != nil {
		return nil, err
	}

	switch config.Name {
	case "swarm", "docker_swarm":
		return dockerswarm.NewDockerSwarmProvider(ctx, logger)
	case "docker":
		return docker.NewDockerClassicProvider(ctx, logger)
	case "kubernetes":
		return kubernetes.NewKubernetesProvider(ctx, logger, config.Kubernetes)
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}

func WatchGroups(ctx context.Context, provider provider.Provider, frequency time.Duration, send chan<- map[string][]string, logger *slog.Logger) {
	ticker := time.NewTicker(frequency)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			groups, err := provider.GetGroups(ctx)
			if err != nil {
				logger.Error("cannot retrieve group from provider", slog.Any("reason", err))
			} else if groups != nil {
				send <- groups
			}
		}
	}
}
