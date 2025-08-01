package nomad

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/nomad/api"
)

type Provider struct {
	Client *api.Client
	l      *slog.Logger
}

func New(ctx context.Context, cli *api.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "nomad"))

	leader, err := cli.Status().Leader()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to nomad api: %v", err)
	}

	peers, err := cli.Status().Peers()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to nomad api: %v", err)
	}

	logger.InfoContext(ctx, "connection established with nomad",
		slog.String("leader", leader),
		slog.Any("peers", peers),
	)
	return &Provider{
		Client: cli,
		l:      logger,
	}, nil
}
