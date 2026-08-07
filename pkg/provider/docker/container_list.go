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
	filters.Add("label", fmt.Sprintf("%s=true", sablier.LabelEnable))

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
	enabled := c.Labels[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(c.Labels[sablier.LabelGroup])
	}

	return sablier.InstanceConfiguration{
		Name:    strings.TrimPrefix(c.Names[0], "/"), // Containers name are reported with a leading slash
		Groups:  groups,
		Enabled: enabled,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	filters := client.Filters{}
	filters.Add("label", fmt.Sprintf("%s=true", sablier.LabelEnable))

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
		name := strings.TrimPrefix(c.Names[0], "/")
		for _, groupName := range sablier.ParseGroups(c.Labels[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], name)
		}
	}

	return groups, nil
}
