package app

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/http/routes"
	"github.com/sablierapp/sablier/app/providers/docker"
	"github.com/sablierapp/sablier/app/providers/dockerswarm"
	"github.com/sablierapp/sablier/app/providers/kubernetes"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sablierapp/sablier/app/providers"
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/app/storage"
	"github.com/sablierapp/sablier/app/theme"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/internal/server"
	"github.com/sablierapp/sablier/version"
	log "github.com/sirupsen/logrus"
)

func Start(ctx context.Context, conf config.Config) error {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logLevel, err := log.ParseLevel(conf.Logging.Level)

	if err != nil {
		log.Warnf("unrecognized log level \"%s\" must be one of [panic, fatal, error, warn, info, debug, trace]", conf.Logging.Level)
		logLevel = log.InfoLevel
	}

	logger := slog.Default()

	log.SetLevel(logLevel)

	log.Info(version.Info())

	provider, err := NewProvider(conf.Provider)
	if err != nil {
		return err
	}

	log.Infof("using provider \"%s\"", conf.Provider.Name)

	store := inmemory.NewInMemory()
	err = store.OnExpire(ctx, onSessionExpires(provider))
	if err != nil {
		return err
	}

	storage, err := storage.NewFileStorage(conf.Storage)
	if err != nil {
		return err
	}

	sessionsManager := sessions.NewSessionsManager(store, provider)
	defer sessionsManager.Stop()

	groups, err := provider.GetGroups(ctx)
	if err != nil {
		log.Warn("could not get groups", err)
	} else {
		sessionsManager.SetGroups(groups)
	}

	updateGroups := make(chan map[string][]string)
	go WatchGroups(ctx, provider, 2*time.Second, updateGroups)
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

	if storage.Enabled() {
		defer saveSessions(storage, sessionsManager)
		loadSessions(storage, sessionsManager)
	}

	if conf.Provider.AutoStopOnStartup {
		err := discovery.StopAllUnregisteredInstances(context.Background(), provider, store)
		if err != nil {
			log.Warnf("Stopping unregistered instances had an error: %v", err)
		}
	}

	var t *theme.Themes

	if conf.Strategy.Dynamic.CustomThemesPath != "" {
		log.Tracef("loading themes with custom theme path: %s", conf.Strategy.Dynamic.CustomThemesPath)
		custom := os.DirFS(conf.Strategy.Dynamic.CustomThemesPath)
		t, err = theme.NewWithCustomThemes(custom)
		if err != nil {
			return err
		}
	} else {
		log.Trace("loading themes without custom themes")
		t, err = theme.New()
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

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Server exiting")

	return nil
}

func onSessionExpires(provider providers.Provider) func(key string) {
	return func(_key string) {
		go func(key string) {
			log.Debugf("stopping %s...", key)
			err := provider.Stop(context.Background(), key)

			if err != nil {
				log.Warnf("error stopping %s: %s", key, err.Error())
			} else {
				log.Debugf("stopped %s", key)
			}
		}(_key)
	}
}

func loadSessions(storage storage.Storage, sessions sessions.Manager) {
	slog.Info("loading sessions from storage")
	reader, err := storage.Reader()
	if err != nil {
		log.Error("error loading sessions", err)
	}
	err = sessions.LoadSessions(reader)
	if err != nil {
		log.Error("error loading sessions", err)
	}
}

func saveSessions(storage storage.Storage, sessions sessions.Manager) {
	slog.Info("writing sessions to storage")
	writer, err := storage.Writer()
	if err != nil {
		log.Error("error saving sessions", err)
		return
	}
	err = sessions.SaveSessions(writer)
	if err != nil {
		log.Error("error saving sessions", err)
	}
}

func NewProvider(config config.Provider) (providers.Provider, error) {
	if err := config.IsValid(); err != nil {
		return nil, err
	}

	switch config.Name {
	case "swarm", "docker_swarm":
		return dockerswarm.NewDockerSwarmProvider()
	case "docker":
		return docker.NewDockerClassicProvider()
	case "kubernetes":
		return kubernetes.NewKubernetesProvider(config.Kubernetes)
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}

func WatchGroups(ctx context.Context, provider providers.Provider, frequency time.Duration, send chan<- map[string][]string) {
	ticker := time.NewTicker(frequency)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			groups, err := provider.GetGroups(ctx)
			if err != nil {
				log.Warn("could not get groups", err)
			} else if groups != nil {
				send <- groups
			}
		}
	}
}
