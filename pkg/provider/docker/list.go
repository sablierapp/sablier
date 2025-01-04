package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (d *DockerProvider) List(ctx context.Context, opts provider.ListOptions) ([]sablier.InstanceConfig, error) {
	args := filters.NewArgs()
	args.Add("label", "sablier.enable")

	found, err := d.Client.ContainerList(ctx, container.ListOptions{
		Filters: args,
		All:     opts.All,
	})
	if err != nil {
		return nil, err
	}

	// d.log.Trace().Msgf("found [%d] containers", len(found))
	infos := make([]sablier.InstanceConfig, 0, len(found))
	for _, c := range found {
		// d.log.Trace().Any("container", c).Msg("container details")
		registered, ok := c.Labels["sablier.enable"]
		if !ok {
			continue
		}
		if !(registered == "" || registered == "true" || registered == "yes") {
			continue
		}

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
