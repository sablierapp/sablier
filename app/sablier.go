package app

import (
	"github.com/acouvreur/sablier/app/http"
	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/app/storage"
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

	provider, err := providers.NewProvider(conf.Provider)
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

	http.Start(conf.Server, conf.Strategy, sessionsManager)

	return nil
}

func onSessionExpires(provider providers.Provider) func(key string, instance instance.State) {
	return func(_key string, _instance instance.State) {
		go func(key string, instance instance.State) {
			log.Debugf("stopping %s...", key)
			_, err := provider.Stop(key)

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
