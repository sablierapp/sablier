package dockerswarm

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *DockerSwarmProvider) InstanceList(ctx context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))
	args.Add("mode", "replicated")

	services, err := p.Client.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: args,
	})

	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(services))
	for _, s := range services {
		instance := p.serviceToInstance(s)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *DockerSwarmProvider) serviceToInstance(s swarm.Service) (i sablier.InstanceConfiguration) {
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

func (p *DockerSwarmProvider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	services, err := p.Client.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: f,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, service := range services {
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
