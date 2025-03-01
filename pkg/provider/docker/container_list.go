package docker

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
	"strings"
)

func (p *DockerClassicProvider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]types.Instance, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     options.All,
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(containers))
	for _, c := range containers {
		instance := containerToInstance(c)
		instances = append(instances, instance)
	}

	return instances, nil
}

func containerToInstance(c dockertypes.Container) types.Instance {
	var group string

	if _, ok := c.Labels[discovery.LabelEnable]; ok {
		if g, ok := c.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}
	}

	return types.Instance{
		Name:  strings.TrimPrefix(c.Names[0], "/"), // Containers name are reported with a leading slash
		Group: group,
	}
}

func (p *DockerClassicProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, c := range containers {
		groupName := c.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}
		group := groups[groupName]
		group = append(group, strings.TrimPrefix(c.Names[0], "/"))
		groups[groupName] = group
	}

	return groups, nil
}
