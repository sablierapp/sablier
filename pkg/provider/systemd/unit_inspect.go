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

	return p.infoFromUnit(unit), nil
}

func (p *Provider) infoFromUnit(unit Unit) sablier.InstanceInfo {
	name := unit.status.Name
	var info sablier.InstanceInfo
	switch unit.status.ActiveState {
	case "inactive", "deactivating", "maintenance":
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusStopped,
		}
	case "activating", "reloading", "refreshing":
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusStarting,
		}
	case "active":
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 1,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusReady,
		}
	case "failed":
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusError,
			Message:         "systemd unit failed",
		}
	default:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusError,
			Message:         fmt.Sprintf("systemd unit status %q not handled", unit.status.ActiveState),
		}
	}

	info.Provider = sablier.ProviderSystemd
	sablier.PopulateEnabledAndGroup(&info, unit.labels)
	return info
}

func (p *Provider) getUnit(ctx context.Context, name string) (Unit, error) {
	unitStatuses, err := p.Con.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return Unit{}, err
	}
	if len(unitStatuses) == 0 || unitStatuses[0].LoadState == "not-found" {
		return Unit{}, fmt.Errorf("unit %q not found", name)
	}

	labels, err := p.getLabels(ctx, name)
	if err != nil {
		return Unit{}, err
	}

	return Unit{
		status: unitStatuses[0],
		labels: labels,
	}, nil
}
