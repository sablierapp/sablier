package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (d *DockerProvider) List(ctx context.Context, opts provider.ListOptions) ([]sablier.InstanceConfig, error) {
	args := filters.NewArgs()
	args.Add("label", "sablier.enable")
	args.Add("label", "sablier.enable=true")

	found, err := d.Client.ContainerList(ctx, container.ListOptions{
		Filters: args,
		All:     opts.All,
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("found %d containers\n", len(found))
	infos := make([]sablier.InstanceConfig, 0, len(found))
	for _, c := range found {
		fmt.Printf("container: %v", c)
		group, ok := c.Labels["sablier.group"]
		if !ok || group == "" {
			group = FormatName(c.Names[0]) // Group defaults to the container name
		}
		conf := sablier.InstanceConfig{
			Name:            FormatName(c.Names[0]),
			Group:           group,
			DesiredReplicas: 1,
		}
		infos = append(infos, conf)
	}

	return infos, nil
}
