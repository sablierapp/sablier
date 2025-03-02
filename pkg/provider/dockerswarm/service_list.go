package dockerswarm

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
)

func (p *DockerSwarmProvider) InstanceList(ctx context.Context, _ provider.InstanceListOptions) ([]types.Instance, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))
	args.Add("mode", "replicated")

	services, err := p.Client.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: args,
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(services))
	for _, s := range services {
		instance := p.serviceToInstance(s)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *DockerSwarmProvider) serviceToInstance(s swarm.Service) (i types.Instance) {
	var group string

	if _, ok := s.Spec.Labels[discovery.LabelEnable]; ok {
		if g, ok := s.Spec.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}
	}

	return types.Instance{
		Name:  s.Spec.Name,
		Group: group,
	}
}

func (p *DockerSwarmProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	services, err := p.Client.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: f,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, service := range services {
		groupName := service.Spec.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		group = append(group, service.Spec.Name)
		groups[groupName] = group
	}

	return groups, nil
}
