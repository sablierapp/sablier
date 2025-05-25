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

	if _, ok := u.props["Enable"]; ok {
		if g, ok := u.props["Group"]; ok {
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
		groupName := u.props["Group"]
		if len(groupName) == 0 {
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
		unitStatuses, err = p.Con.ListUnitsContext(ctx)
	} else {
		unitStatuses, err = p.Con.ListUnitsFilteredContext(ctx, []string{"active"})
	}
	if err != nil {
		return nil, err
	}

	units := make([]Unit, 0)
	for _, unitStatus := range unitStatuses {
		sablierProps, err := p.parseSablierProperties(unitStatus)
		if err != nil {
			return nil, err
		}
		if sablierProps["Enable"] == "true" {
			units = append(units, Unit{
				status: unitStatus,
				props:  sablierProps,
			})
		}
	}

	return units, nil
}

type Unit struct {
	status dbus.UnitStatus
	props  map[string]string
}
