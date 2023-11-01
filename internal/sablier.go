package app

import (
	"context"
	"fmt"
	"github.com/acouvreur/sablier/internal/api"
	"github.com/acouvreur/sablier/internal/theme"
	log "log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/provider/docker"
	"github.com/acouvreur/sablier/internal/provider/kubernetes"
	"github.com/acouvreur/sablier/internal/provider/swarm"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/version"
)

func Start(conf config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info(version.Info())

	// Load themes
	t, err := NewThemes(conf.Strategy.Dynamic.CustomThemesPath)

	// Create client/provider
	client, err := NewClient(conf.Provider)
	if err != nil {
		return err
	}
	log.Info("using provider \"%s\"", conf.Provider.Name)

	// Create the session manager
	sm := session.NewManager(client, conf.Sessions)

	// Create the auto discovery for groups
	d := provider.NewDiscovery(client, provider.DiscoveryOptions{
		EnableLabel:          provider.EnableLabel,
		GroupLabel:           provider.GroupLabel,
		DefaultGroupStrategy: provider.DefaultGroupStrategyUseValue,
		StopOnDiscover:       false,
	})
	go d.StartDiscovery(ctx)

	// Start the api
	api.Start(ctx, conf, t, sm, d)

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

	default:
		return nil, fmt.Errorf("unknown provider %s", conf.Name)
	}
}

func NewThemes(path string) (*theme.Themes, error) {
	if path != "" {
		ct := os.DirFS(path)
		return theme.NewWithCustomThemes(ct)
	}

	return theme.New(), nil
}
