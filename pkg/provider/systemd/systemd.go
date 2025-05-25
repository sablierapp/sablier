package systemd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Con             *dbus.Conn
	desiredReplicas int32
	l               *slog.Logger
}

func New(ctx context.Context, con *dbus.Conn, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "systemd"))

	connected := con.Connected()
	if !connected {
		return nil, fmt.Errorf("no connection to systemd dbus")
	}

	logger.InfoContext(ctx, "connection established with systemd dbus")
	return &Provider{
		Con: con,
		l:   logger,
	}, nil
}
