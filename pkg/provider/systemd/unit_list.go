package systemd

import (
	"context"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	units, err := p.listUnits(ctx, options)
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(units))
	for _, u := range units {
		instances = append(instances, unitToInstance(u))
	}

	return instances, nil
}

func unitToInstance(u Unit) sablier.InstanceConfiguration {
	var group string

	if _, ok := u.props["X-Sablier-Enable"]; ok {
		if g, ok := u.props["X-Sablier-Group"].(string); ok {
			group = g
		} else {
			group = "default"
		}
	}

	return sablier.InstanceConfiguration{
		Name:  u.status.Name,
		Group: group,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	units, err := p.listUnits(ctx, provider.InstanceListOptions{
		All: true,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, u := range units {
		groupName, ok := u.props["X-Sablier-Group"].(string)
		if !ok || len(groupName) == 0 {
			groupName = "default"
		}
		group := groups[groupName]
		group = append(group, u.status.Name)
		groups[groupName] = group
	}

	return groups, nil
}

func (p *Provider) listUnits(ctx context.Context, options provider.InstanceListOptions) ([]Unit, error) {
	var unitStatuses []dbus.UnitStatus
	var err error
	if options.All {
		unitStatuses, err = p.Con.ListUnitsFilteredContext(ctx, []string{"active"})
	} else {
		unitStatuses, err = p.Con.ListUnitsContext(ctx)
	}
	if err != nil {
		return nil, err
	}

	units := make([]Unit, 0, len(unitStatuses))
	for _, unitStatus := range unitStatuses {
		props, err := p.Con.GetUnitPropertiesContext(ctx, unitStatus.Name)
		if err != nil {
			return nil, err
		}
		if props["X-Sablier-Enable"] == "true" {
			units = append(units, Unit{
				status: unitStatus,
				props:  props,
			})
		}
	}

	return units, nil
}

type Unit struct {
	status dbus.UnitStatus
	props  map[string]interface{}
}
