package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/sablier"
)

var _ sablier.Provider = (*DockerProvider)(nil)

type DockerProvider struct {
	Client *client.Client

	UseCheckpoint bool
	UsePause      bool

	log zerolog.Logger
}

func NewDockerProvider(cli *client.Client, logger zerolog.Logger) (*DockerProvider, error) {
	logger = logger.With().Str("provider", "docker").
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
