package systemd

import (
	"context"
	"log/slog"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

// NewForTest creates a Provider with a custom poll interval for testing.
func NewForTest(ctx context.Context, con *dbus.Conn, logger *slog.Logger, pollInterval time.Duration) (*Provider, error) {
	p, err := New(ctx, con, logger, nil)
	if err != nil {
		return nil, err
	}
	p.pollInterval = pollInterval
	return p, nil
}

func (p *Provider) UnitTypePropertiesForTest(ctx context.Context, name, unitType string) (map[string]any, error) {
	return p.Con.GetUnitTypePropertiesContext(ctx, name, unitType)
}
