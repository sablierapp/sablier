package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	filters := client.Filters{}
	filters.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	p.l.DebugContext(ctx, "listing containers", slog.Group("options", slog.Bool("all", options.All), slog.Any("filters", filters)))
	containers, err := p.Client.ContainerList(ctx, client.ContainerListOptions{
		All:     options.All,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(containers.Items)))

	instances := make([]sablier.InstanceConfiguration, 0, len(containers.Items))
	for _, c := range containers.Items {
		instance := containerToInstance(c)
		instances = append(instances, instance)
	}

	return instances, nil
}

func containerToInstance(c container.Summary) sablier.InstanceConfiguration {
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
	filters := client.Filters{}
	filters.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	p.l.DebugContext(ctx, "listing containers", slog.Group("options", slog.Bool("all", true), slog.Any("filters", filters)))
	containers, err := p.Client.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(containers.Items)))

	groups := make(map[string][]string)
	for _, c := range containers.Items {
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
