package podman

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	args := &containers.ListOptions{
		All:     &options.All,
		Filters: map[string][]string{"label": {fmt.Sprintf("%s=true", "sablier.enable")}},
	}

	p.l.DebugContext(ctx, "listing containers", slog.Group("options", slog.Bool("all", options.All), slog.Any("filters", args)))
	found, err := containers.List(p.conn, args)
	if err != nil {
		return nil, fmt.Errorf("error listing containers: %v", err)
	}

	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(found)), slog.Any("containers", found))

	instances := make([]sablier.InstanceConfiguration, 0, len(found))
	for _, c := range found {
		instance := containerToInstance(c)
		instances = append(instances, instance)
	}

	return instances, nil
}

func containerToInstance(c types.ListContainer) sablier.InstanceConfiguration {
	var group string

	if _, ok := c.Labels["sablier.enable"]; ok {
		if g, ok := c.Labels["sablier.group"]; ok {
			group = g
		} else {
			group = "default"
		}
	}

	return sablier.InstanceConfiguration{
		Name:  strings.TrimPrefix(c.Names[0], "/"), // Containers name are reported with a leading slash
		Group: group,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	all := true
	args := &containers.ListOptions{
		All:     &all,
		Filters: map[string][]string{"label": {fmt.Sprintf("%s=true", "sablier.enable")}},
	}

	p.l.DebugContext(ctx, "listing containers", slog.Group("options", slog.Bool("all", all), slog.Any("filters", args)))
	found, err := containers.List(p.conn, args)
	if err != nil {
		return nil, fmt.Errorf("error listing containers: %v", err)
	}
	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(found)), slog.Any("containers", found))

	groups := make(map[string][]string)
	for _, c := range found {
		groupName := c.Labels["sablier.group"]
		if len(groupName) == 0 {
			groupName = "default"
		}
		group := groups[groupName]
		group = append(group, strings.TrimPrefix(c.Names[0], "/"))
		groups[groupName] = group
	}

	return groups, nil
}
