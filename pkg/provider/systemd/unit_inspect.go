package systemd

import (
	"context"
	"fmt"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	unit, err := p.getUnit(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot inspect systemd unit: %w", err)
	}

	// "active", "inactive", "failed", "activating", "deactivating", "maintenance", "reloading" or "refreshing"
	switch unit.status.ActiveState {
	case "inactive", "deactivating", "maintenance", "reloading", "refreshing":
		return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "active":
		return sablier.ReadyInstanceState(name, p.desiredReplicas), nil
	case "failed":
		return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("system unit failed"), p.desiredReplicas), nil
	default:
		return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("systemd unit status \"%s\" not handled", unit.status.ActiveState), p.desiredReplicas), nil
	}
}

func (p *Provider) getUnit(ctx context.Context, name string) (Unit, error) {
	unitStatuses, err := p.Con.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return Unit{}, err
	}

	if len(unitStatuses) == 0 {
		return Unit{}, fmt.Errorf("unit %s not found", name)
	}

	unitStatus := unitStatuses[0]
	props, err := p.Con.GetUnitPropertiesContext(ctx, unitStatus.Name)
	if err != nil {
		return Unit{}, err
	}

	return Unit{
		status: unitStatus,
		props:  props,
	}, nil
}
