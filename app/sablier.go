package app

import (
	"github.com/ThreeDotsLabs/watermill/message"
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/providers/docker"
	"github.com/sablierapp/sablier/app/providers/dockerswarm"
	"github.com/sablierapp/sablier/app/providers/kubernetes"
	"github.com/sablierapp/sablier/pkg/promise"
	"os"
	"sync"

	"github.com/sablierapp/sablier/app/http"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/providers"
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/app/storage"
	"github.com/sablierapp/sablier/app/theme"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/tinykv"
	"github.com/sablierapp/sablier/version"
	log "github.com/sirupsen/logrus"
)

// TODO:
// For blocking: retry policy
// For retrieving result: global timeout
// Publisher / Reconcile loop with providers

type PubSub interface {
	message.Publisher
	message.Subscriber
}

type Sablier struct {
	provider    Provider
	promises    map[string]*promise.Promise[Instance]
	expirations tinykv.KV[string]

	pubsub PubSub

	lock sync.RWMutex
}

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
