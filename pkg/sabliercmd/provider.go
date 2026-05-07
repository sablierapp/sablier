package sabliercmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/sablier"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func setupProvider(ctx context.Context, logger *slog.Logger, config config.Provider) (sablier.Provider, error) {
	if err := config.IsValid(); err != nil {
		return nil, err
	}

	switch config.Name {
	case "swarm", "docker_swarm":
		cli, err := client.New(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("cannot create docker swarm client: %v", err)
		}
		return dockerswarm.New(ctx, cli, logger)
	case "docker":
		cli, err := client.New(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("cannot create docker client: %v", err)
		}
		return docker.New(ctx, cli, logger, config.Docker.Strategy)
	case "kubernetes":
		kubeclientConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		kubeclientConfig.QPS = config.Kubernetes.QPS
		kubeclientConfig.Burst = config.Kubernetes.Burst

		cli, err := k8s.NewForConfig(kubeclientConfig)
		if err != nil {
			return nil, err
		}
		return kubernetes.New(ctx, cli, logger, config.Kubernetes)
	case "podman":
		opts := []client.Opt{client.FromEnv}
		if config.Podman.Uri != "" {
			opts = append(opts, client.WithHost(config.Podman.Uri))
		}
		cli, err := client.New(opts...)
		if err != nil {
			return nil, fmt.Errorf("cannot create podman client: %w", err)
		}
		return podman.New(ctx, cli, logger)
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}
