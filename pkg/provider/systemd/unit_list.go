package systemd

import (
	"cmp"
	"context"
	"log/slog"
	"slices"

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
	enabled := u.labels[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(u.labels[sablier.LabelGroup])
	}

	return sablier.InstanceConfiguration{
		Name:    u.status.Name,
		Groups:  groups,
		Enabled: enabled,
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
		for _, groupName := range sablier.ParseGroups(u.labels[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], u.status.Name)
		}
	}

	return groups, nil
}

func (p *Provider) listUnits(ctx context.Context, options provider.InstanceListOptions) ([]Unit, error) {
	if options.All {
		return p.listUnitFiles(ctx)
	}

	unitStatuses, err := p.Con.ListUnitsFilteredContext(ctx, []string{"active"})
	if err != nil {
		return nil, err
	}

	units := make([]Unit, 0)
	for _, unitStatus := range unitStatuses {
		labels, err := p.readLabels(ctx, unitStatus.Name)
		if err != nil {
			p.l.DebugContext(ctx, "cannot read unit file, skipping unit", slog.String("name", unitStatus.Name), slog.Any("error", err))
			continue
		}
		if labels[sablier.LabelEnable] == "true" {
			units = append(units, Unit{
				status: unitStatus,
				labels: labels,
			})
		}
	}

	return units, nil
}

func (p *Provider) listUnitFiles(ctx context.Context) ([]Unit, error) {
	files, err := p.Con.ListUnitFilesContext(ctx)
	if err != nil {
		return nil, err
	}

	units := make([]Unit, 0)
	seen := make(map[string]struct{})
	for _, file := range files {
		name := unitNameFromPath(file.Path)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}

		labels, err := labelsFromUnitFile(file.Path)
		if err != nil {
			p.l.DebugContext(ctx, "cannot read unit file, skipping unit", slog.String("name", name), slog.Any("error", err))
			continue
		}
		if labels[sablier.LabelEnable] != "true" {
			continue
		}
		seen[name] = struct{}{}
		units = append(units, Unit{
			status: dbus.UnitStatus{Name: name},
			labels: labels,
		})
	}

	slices.SortFunc(units, func(a, b Unit) int { return cmp.Compare(a.status.Name, b.status.Name) })
	return units, nil
}

type Unit struct {
	status dbus.UnitStatus
	labels map[string]string
}
