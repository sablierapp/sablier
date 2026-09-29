package awsecs

import (
	"context"
	"log/slog"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceList(ctx context.Context, opts provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	services, err := p.listEnabledServices(ctx)
	if err != nil {
		return nil, err
	}

	p.l.DebugContext(ctx, "listing instances", slog.Bool("all", opts.All), slog.Int("services", len(services)))

	instances := make([]sablier.InstanceConfiguration, 0, len(services))
	for _, svc := range services {
		// A service with no desired task is stopped. Only list it when All is set.
		if !opts.All && svc.DesiredCount == 0 {
			continue
		}
		instances = append(instances, serviceToInstance(svc))
	}
	return instances, nil
}

func serviceToInstance(svc types.Service) sablier.InstanceConfiguration {
	labels := tagsToLabels(svc.Tags)
	enabled := labels[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(labels[sablier.LabelGroup])
	}
	return sablier.InstanceConfiguration{
		Name:    aws.ToString(svc.ServiceName),
		Groups:  groups,
		Enabled: enabled,
	}
}

// groupsOf returns the groups of an enabled service.
func groupsOf(svc types.Service) []string {
	return sablier.ParseGroups(tagsToLabels(svc.Tags)[sablier.LabelGroup])
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	services, err := p.listEnabledServices(ctx)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, svc := range services {
		name := aws.ToString(svc.ServiceName)
		for _, g := range groupsOf(svc) {
			groups[g] = append(groups[g], name)
		}
	}
	for _, names := range groups {
		slices.Sort(names)
	}
	return groups, nil
}
