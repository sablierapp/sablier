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
	"log/slog"
	"strconv"
)

func (p *DockerSwarmProvider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]types.Instance, error) {
	args := filters.NewArgs()
	for _, label := range options.Labels {
		args.Add("label", label)
		args.Add("label", fmt.Sprintf("%s=true", label))
	}

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
	var replicas uint64

	if _, ok := s.Spec.Labels[discovery.LabelEnable]; ok {
		if g, ok := s.Spec.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}

		if r, ok := s.Spec.Labels[discovery.LabelReplicas]; ok {
			atoi, err := strconv.Atoi(r)
			if err != nil {
				p.l.Warn("invalid replicas label value, using default replicas value", slog.Any("error", err), slog.String("instance", s.Spec.Name), slog.String("value", r))
				replicas = discovery.LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = discovery.LabelReplicasDefaultValue
		}
	}

	return types.Instance{
		Name: s.Spec.Name,
		Kind: "service",
		// TODO
		// Status:          string(s.UpdateStatus.State),
		// Replicas:        s.ServiceStatus.RunningTasks,
		// DesiredReplicas: s.ServiceStatus.DesiredTasks,
		ScalingReplicas: replicas,
		Group:           group,
	}
}
