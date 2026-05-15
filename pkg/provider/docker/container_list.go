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

	enabled := c.Labels["sablier.enable"]
	if enabled == "true" {
		if g, ok := c.Labels["sablier.group"]; ok {
			group = g
		} else {
			group = "default"
		}
	}

	return sablier.InstanceConfiguration{
		Name:    strings.TrimPrefix(c.Names[0], "/"), // Containers name are reported with a leading slash
		Group:   group,
		Enabled: enabled,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]sablier.InstanceConfiguration, error) {
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

	// Build a map of Docker Compose service name → container name so that the
	// sablier.depends-on label can reference either a service name or a raw
	// container name.
	serviceToContainer := make(map[string]string, len(containers.Items))
	for _, c := range containers.Items {
		if svc := c.Labels["com.docker.compose.service"]; svc != "" {
			serviceToContainer[svc] = strings.TrimPrefix(c.Names[0], "/")
		}
	}

	// First pass: bucket containers by group and collect per-container deps.
	type containerEntry struct {
		name      string
		dependsOn []string // raw values from label (service or container names)
	}
	rawGroups := make(map[string][]containerEntry)
	for _, c := range containers.Items {
		groupName := c.Labels["sablier.group"]
		if len(groupName) == 0 {
			groupName = "default"
		}
		name := strings.TrimPrefix(c.Names[0], "/")

		var deps []string
		if v := c.Labels["sablier.depends-on"]; v != "" {
			deps = splitCSV(v)
		}

		rawGroups[groupName] = append(rawGroups[groupName], containerEntry{name: name, dependsOn: deps})
	}

	// Second pass: resolve deps to container names, topologically sort, and
	// build the final InstanceConfiguration list per group.
	groups := make(map[string][]sablier.InstanceConfiguration, len(rawGroups))
	for groupName, entries := range rawGroups {
		// Build a set of container names in this group for cross-referencing.
		inGroup := make(map[string]bool, len(entries))
		for _, e := range entries {
			inGroup[e.name] = true
		}

		// Resolve dep names: service label → container name, or use as-is.
		deps := make(map[string][]string, len(entries))
		for _, e := range entries {
			if len(e.dependsOn) == 0 {
				continue
			}
			resolved := make([]string, 0, len(e.dependsOn))
			for _, raw := range e.dependsOn {
				dep := raw
				if mapped, ok := serviceToContainer[raw]; ok {
					dep = mapped
				}
				if inGroup[dep] {
					resolved = append(resolved, dep)
				} else {
					p.l.WarnContext(ctx, "sablier.depends-on references a container not in the same group, ignoring",
						slog.String("container", e.name),
						slog.String("group", groupName),
						slog.String("dependency", raw),
					)
				}
			}
			if len(resolved) > 0 {
				deps[e.name] = resolved
			}
		}

		// Topological sort (dependencies before dependents).
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.name
		}
		sorted, err := topoSort(names, deps)
		if err != nil {
			p.l.WarnContext(ctx, "cannot sort group by depends-on (cycle detected), using original order",
				slog.String("group", groupName),
				slog.Any("error", err),
			)
			sorted = names
		}

		// Build InstanceConfiguration slice in sorted order.
		configs := make([]sablier.InstanceConfiguration, 0, len(sorted))
		for _, name := range sorted {
			configs = append(configs, sablier.InstanceConfiguration{
				Name:      name,
				DependsOn: deps[name],
			})
		}
		groups[groupName] = configs
	}

	return groups, nil
}

// splitCSV splits a comma-separated string and trims whitespace from each element.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
