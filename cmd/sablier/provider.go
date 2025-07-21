package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/provider/systemd"
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
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("cannot create docker swarm client: %v", err)
		}
		return dockerswarm.New(ctx, cli, logger)
	case "docker":
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("cannot create docker client: %v", err)
		}
		return docker.New(ctx, cli, logger)
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
	case "systemd":
		var con *dbus.Conn
		var err error
		if config.Systemd.UserInstance {
			con, err = dbus.NewUserConnectionContext(ctx)
		} else {
			con, err = dbus.NewSystemConnectionContext(ctx)
		}
		if err != nil {
			return nil, err
		}

		return systemd.New(ctx, con, logger)
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}
