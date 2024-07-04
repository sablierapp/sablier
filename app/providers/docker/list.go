package docker

import (
	"context"
	"fmt"
	"github.com/acouvreur/sablier/app/discovery"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/app/types"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"strings"
)

func (provider *DockerClassicProvider) InstanceList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	args := filters.NewArgs()
	for _, label := range options.Labels {
		args.Add("label", label)
		args.Add("label", fmt.Sprintf("%s=true", label))
	}

	containers, err := provider.Client.ContainerList(ctx, container.ListOptions{
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
		Name:   strings.TrimPrefix(c.Names[0], "/"), // Containers name are reported with a leading slash
		Kind:   "container",
		Status: c.Status,
		// Replicas:        c.Status,
		// DesiredReplicas: 1,
		ScalingReplicas: 1,
		Group:           group,
	}
}
