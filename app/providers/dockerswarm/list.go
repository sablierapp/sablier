package dockerswarm

import (
	"context"
	"fmt"
	"github.com/acouvreur/sablier/app/discovery"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/app/types"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	log "github.com/sirupsen/logrus"
	"strconv"
)

func (provider *DockerSwarmProvider) InstanceList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	args := filters.NewArgs()
	for _, label := range options.Labels {
		args.Add("label", label)
		args.Add("label", fmt.Sprintf("%s=true", label))
	}

	services, err := provider.Client.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: args,
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, len(services))
	for _, s := range services {
		instance := serviceToInstance(s)
		instances = append(instances, instance)
	}

	return instances, nil
}

func serviceToInstance(s swarm.Service) (i types.Instance) {
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
				log.Warnf("Defaulting to default replicas value, could not convert value \"%v\" to int: %v", r, err)
				replicas = discovery.LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = discovery.LabelReplicasDefaultValue
		}
	}

	return types.Instance{
		Name:            s.Spec.Name,
		Kind:            "service",
		Status:          string(s.UpdateStatus.State),
		Replicas:        s.ServiceStatus.RunningTasks,
		DesiredReplicas: s.ServiceStatus.DesiredTasks,
		ScalingReplicas: replicas,
		Group:           group,
	}
}
