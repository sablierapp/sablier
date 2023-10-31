package app

import (
	log "log/slog"

	"github.com/acouvreur/sablier/app/http"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/provider/docker"
	"github.com/acouvreur/sablier/internal/provider/kubernetes"
	"github.com/acouvreur/sablier/internal/provider/swarm"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/version"
)

func Start(conf config.Config) error {

	log.Info(version.Info())

	client, err := NewClient(conf.Provider)
	if err != nil {
		return err
	}
	log.Info("using provider \"%s\"", conf.Provider.Name)

	sessionsManager := session.NewSessionManager(client, conf.Sessions)

	http.Start(conf.Server, conf.Strategy, conf.Sessions, sessionsManager)

	return nil
}

func NewClient(conf config.Provider) (provider.Client, error) {
	switch conf.Name {
	case config.Docker:
		return docker.NewDockerClient()
	case config.Swarm:
		return swarm.NewSwarmClient()
	case config.Kubernetes:
		return kubernetes.NewKubernetesClient()
	}
}
