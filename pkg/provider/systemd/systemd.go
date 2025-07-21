package systemd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gopkg.in/ini.v1"
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

func (p *Provider) parseSablierProperties(unitStatus dbus.UnitStatus) (map[string]string, error) {
	dbusProps, err := p.Con.GetUnitPropertiesContext(context.Background(), unitStatus.Name)
	if err != nil {
		return nil, err
	}

	sourcePath, ok := dbusProps["SourcePath"].(string)
	if !ok || sourcePath == "" {
		// Not a unit we could start or stop
		return nil, nil
	}

	cfg, err := ini.Load(sourcePath)
	if err != nil {
		return nil, err
	}

	section, err := cfg.GetSection("X-Sablier")
	if err != nil {
		// No sablier props found
		return nil, nil
	}

	props := make(map[string]string)
	for _, key := range section.Keys() {
		props[key.Name()] = key.Value()
	}

	return props, nil
}
