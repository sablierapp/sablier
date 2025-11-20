package digitalocean

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/digitalocean/godo"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	p.l.DebugContext(ctx, "listing apps", slog.Group("options", slog.Bool("all", options.All)))

	// List all apps
	apps, _, err := p.Client.Apps.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot list apps: %w", err)
	}

	p.l.DebugContext(ctx, "apps listed", slog.Int("count", len(apps)))

	instances := make([]sablier.InstanceConfiguration, 0, len(apps))
	for _, app := range apps {
		// Filter by sablier.enable label if not listing all
		if !options.All {
			enabled := false
			for _, env := range app.Spec.Envs {
				if env.Key == "SABLIER_ENABLE" && env.Value == "true" {
					enabled = true
					break
				}
			}
			if !enabled {
				continue
			}
		}

		instance := appToInstance(app)
		instances = append(instances, instance)
	}

	return instances, nil
}

func appToInstance(app *godo.App) sablier.InstanceConfiguration {
	var group string = "default"

	// Look for sablier.group in environment variables
	for _, env := range app.Spec.Envs {
		if env.Key == "SABLIER_GROUP" {
			group = env.Value
			break
		}
	}

	return sablier.InstanceConfiguration{
		Name:  app.ID,
		Group: group,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	p.l.DebugContext(ctx, "listing apps for groups")

	// List all apps
	apps, _, err := p.Client.Apps.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot list apps: %w", err)
	}

	p.l.DebugContext(ctx, "apps listed", slog.Int("count", len(apps)))

	groups := make(map[string][]string)
	for _, app := range apps {
		// Only include apps with sablier.enable
		enabled := false
		groupName := "default"

		for _, env := range app.Spec.Envs {
			if env.Key == "SABLIER_ENABLE" && env.Value == "true" {
				enabled = true
			}
			if env.Key == "SABLIER_GROUP" {
				groupName = env.Value
			}
		}

		if !enabled {
			continue
		}

		group := groups[groupName]
		group = append(group, app.ID)
		groups[groupName] = group
	}

	return groups, nil
}
