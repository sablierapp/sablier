package digitalocean

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/digitalocean/godo"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client          *godo.Client
	desiredReplicas int32
	l               *slog.Logger
}

func New(ctx context.Context, client *godo.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "digitalocean"))

	// Test connection by getting account info
	account, _, err := client.Account.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Digital Ocean API: %v", err)
	}

	logger.InfoContext(ctx, "connection established with Digital Ocean",
		slog.String("email", account.Email),
		slog.String("status", account.Status),
	)

	return &Provider{
		Client:          client,
		desiredReplicas: 1,
		l:               logger,
	}, nil
}
