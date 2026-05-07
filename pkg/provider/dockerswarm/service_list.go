package dockerswarm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	filters := client.Filters{}
	filters.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))
	filters.Add("mode", "replicated")

	p.l.DebugContext(ctx, "listing services", slog.Group("options", slog.Bool("status", true), slog.Any("filters", filters)))
	services, err := p.Client.ServiceList(ctx, client.ServiceListOptions{
		Status:  true,
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot list services: %w", err)
	}
	p.l.DebugContext(ctx, "services listed", slog.Int("count", len(services.Items)), slog.Any("services", services))

	instances := make([]sablier.InstanceConfiguration, 0, len(services.Items))
	for _, s := range services.Items {
		instance := p.serviceToInstance(s)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *Provider) serviceToInstance(s swarm.Service) (i sablier.InstanceConfiguration) {
	var group string

	if _, ok := s.Spec.Labels["sablier.enable"]; ok {
		if g, ok := s.Spec.Labels["sablier.group"]; ok {
			group = g
		} else {
			group = "default"
		}
	}

	return sablier.InstanceConfiguration{
		Name:  s.Spec.Name,
		Group: group,
	}
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	filters := client.Filters{}
	filters.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))
	p.l.DebugContext(ctx, "listing services", slog.Group("options", slog.Bool("status", true), slog.Any("filters", filters)))
	services, err := p.Client.ServiceList(ctx, client.ServiceListOptions{
		Status:  true,
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot list services: %w", err)
	}

	p.l.DebugContext(ctx, "services listed", slog.Int("count", len(services.Items)))

	groups := make(map[string][]string)
	for _, service := range services.Items {
		groupName := service.Spec.Labels["sablier.group"]
		if len(groupName) == 0 {
			groupName = "default"
		}

		group := groups[groupName]
		group = append(group, service.Spec.Name)
		groups[groupName] = group
	}

	return groups, nil
}
