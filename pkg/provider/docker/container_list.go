package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	p.l.DebugContext(ctx, "listing containers", slog.Group("options", slog.Bool("all", options.All), slog.Any("filters", args)))
	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     options.All,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(containers)))

	instances := make([]sablier.InstanceConfiguration, 0, len(containers))
	for _, c := range containers {
		instance := containerToInstance(c, p.composeAutoGroup)
		instances = append(instances, instance)
	}

	return instances, nil
}

func containerToInstance(c container.Summary, composeAutoGroup bool) sablier.InstanceConfiguration {
	var group string

	if _, ok := c.Labels["sablier.enable"]; ok {
		if g, ok := c.Labels["sablier.group"]; ok {
			group = g
		} else if composeAutoGroup {
			if project, ok := c.Labels[ComposeProjectLabel]; ok && project != "" {
				group = project
			}
		}
		if group == "" {
			group = "default"
		}
	}

	return sablier.InstanceConfiguration{
		Name:  strings.TrimPrefix(c.Names[0], "/"),
		Group: group,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	p.l.DebugContext(ctx, "listing containers", slog.Group("options", slog.Bool("all", true), slog.Any("filters", args)))
	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	p.l.DebugContext(ctx, "containers listed", slog.Int("count", len(containers)))

	groups := make(map[string][]string)
	for _, c := range containers {
		groupName := c.Labels["sablier.group"]
		if len(groupName) == 0 {
			if p.composeAutoGroup {
				if project, ok := c.Labels[ComposeProjectLabel]; ok && project != "" {
					groupName = project
				}
			}
		}
		if len(groupName) == 0 {
			groupName = "default"
		}
		group := groups[groupName]
		group = append(group, strings.TrimPrefix(c.Names[0], "/"))
		groups[groupName] = group
	}

	return groups, nil
}

func (p *Provider) InstanceDependencies(ctx context.Context) (map[string][]string, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	// Build a mapping of compose service name → container name, scoped by project.
	// Key: "project/service", Value: container name.
	serviceToContainer := make(map[string]string)
	for _, c := range containers {
		project := c.Labels[ComposeProjectLabel]
		service := c.Labels[ComposeServiceLabel]
		if project != "" && service != "" {
			serviceToContainer[project+"/"+service] = strings.TrimPrefix(c.Names[0], "/")
		}
	}

	deps := make(map[string][]string)
	for _, c := range containers {
		compose := parseComposeLabels(c.Labels)
		if len(compose.DependsOn) == 0 {
			continue
		}

		containerName := strings.TrimPrefix(c.Names[0], "/")
		var resolved []string
		for _, dep := range compose.DependsOn {
			key := compose.Project + "/" + dep.Service
			if depName, ok := serviceToContainer[key]; ok {
				resolved = append(resolved, depName)
			} else {
				p.l.DebugContext(ctx, "dependency not found among sablier-enabled containers",
					slog.String("container", containerName),
					slog.String("dependency_service", dep.Service),
					slog.String("project", compose.Project),
				)
			}
		}
		if len(resolved) > 0 {
			deps[containerName] = resolved
		}
	}

	return deps, nil
}
