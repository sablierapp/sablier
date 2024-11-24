package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/checkpoint"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"os"
	"time"
)

var _ sablier.Provider = (*DockerProvider)(nil)

type DockerProvider struct {
	Client *client.Client

	UseCheckpoint bool
	UsePause      bool

	log zerolog.Logger
}

func NewDockerProvider(cli *client.Client) (*DockerProvider, error) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().
		Str("provider", "docker").
		Logger()
	serverVersion, err := cli.ServerVersion(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %v", err)
	}

	logger.Debug().
		Str("server-version", serverVersion.Version).
		Str("api-version", serverVersion.APIVersion).
		Msg("connection established with docker")

	return &DockerProvider{
		Client:        cli,
		UseCheckpoint: false,
		UsePause:      false,
		log:           logger,
	}, nil
}

func (d *DockerProvider) Stop(ctx context.Context, name string) error {
	if d.UsePause {
		return d.Client.ContainerPause(ctx, name)
	}

	if d.UseCheckpoint {
		return d.Client.CheckpointCreate(ctx, name, checkpoint.CreateOptions{
			CheckpointID: name,
			Exit:         true,
		})
	}

	return d.Client.ContainerStop(ctx, name, container.StopOptions{})
}

func (d *DockerProvider) Info(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	return sablier.InstanceInfo{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: 0,
		Status:          sablier.InstanceStarting,
		StartedAt:       time.Now(),
	}, nil
}

func (d *DockerProvider) List(ctx context.Context, opts provider.ListOptions) ([]sablier.InstanceConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DockerProvider) Events(ctx context.Context) (<-chan sablier.Message, <-chan error) {
	//TODO implement me
	panic("implement me")
}
