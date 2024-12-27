package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/sablier"
	"os"
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

func (d *DockerProvider) Events(ctx context.Context) (<-chan sablier.Message, <-chan error) {
	//TODO implement me
	panic("implement me")
}
