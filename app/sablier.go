package app

import (
	"context"
	"fmt"
	"github.com/acouvreur/sablier/app/discovery"
	"github.com/acouvreur/sablier/app/providers/docker"
	"github.com/acouvreur/sablier/app/providers/dockerswarm"
	"github.com/acouvreur/sablier/app/providers/kubernetes"
	"os"

	"github.com/acouvreur/sablier/app/http"
	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/app/storage"
	"github.com/acouvreur/sablier/app/theme"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/pkg/tinykv"
	"github.com/acouvreur/sablier/version"
	log "github.com/sirupsen/logrus"
)

func Start(conf config.Config) error {

	logLevel, err := log.ParseLevel(conf.Logging.Level)

	if err != nil {
		log.Warnf("unrecognized log level \"%s\" must be one of [panic, fatal, error, warn, info, debug, trace]", conf.Logging.Level)
		logLevel = log.InfoLevel
	}

	log.SetLevel(logLevel)

	log.Info(version.Info())

	provider, err := NewProvider(conf.Provider)
	if err != nil {
		return err
	}

	log.Infof("using provider \"%s\"", conf.Provider.Name)

	store := tinykv.New(conf.Sessions.ExpirationInterval, onSessionExpires(provider))

	storage, err := storage.NewFileStorage(conf.Storage)
	if err != nil {
		return err
	}

	sessionsManager := sessions.NewSessionsManager(store, provider)
	defer sessionsManager.Stop()

	if storage.Enabled() {
		defer saveSessions(storage, sessionsManager)
		loadSessions(storage, sessionsManager)
	}

	if conf.Provider.AutoStopOnStartup {
		err := discovery.StopAllUnregisteredInstances(context.Background(), provider, store.Keys())
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

	http.Start(conf.Server, conf.Strategy, conf.Sessions, sessionsManager, t)

	return nil
}

func onSessionExpires(provider providers.Provider) func(key string, instance instance.State) {
	return func(_key string, _instance instance.State) {
		go func(key string, instance instance.State) {
			log.Debugf("stopping %s...", key)
			err := provider.Stop(context.Background(), key)

			if err != nil {
				log.Warnf("error stopping %s: %s", key, err.Error())
			} else {
				log.Debugf("stopped %s", key)
			}
		}(_key, _instance)
	}
}

func loadSessions(storage storage.Storage, sessions sessions.Manager) {
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
