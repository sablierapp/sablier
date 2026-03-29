package proxmoxlxc

import (
	"context"
	"log/slog"
	"sort"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	discovered, err := p.scanContainers(ctx)
	if err != nil {
		return nil, err
	}

	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(discovered)), slog.Bool("all", options.All))

	instances := make([]sablier.InstanceConfiguration, 0, len(discovered))
	for _, d := range discovered {
		if !options.All && d.status != "running" {
			continue
		}
		instances = append(instances, sablier.InstanceConfiguration{
			Name:  d.ref.name,
			Group: extractGroup(d.tags),
		})
	}

	return instances, nil
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	discovered, err := p.scanContainers(ctx)
	if err != nil {
		return nil, err
	}

	p.l.DebugContext(ctx, "containers listed for groups", slog.Int("count", len(discovered)))

	groups := make(map[string][]string)
	for _, d := range discovered {
		groupName := extractGroup(d.tags)
		groups[groupName] = append(groups[groupName], d.ref.name)
	}

	// Sort instance names within each group for stable ordering
	for _, names := range groups {
		sort.Strings(names)
	}

	return groups, nil
}
