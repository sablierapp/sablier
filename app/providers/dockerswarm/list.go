package dockerswarm

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/types"
	log "github.com/sirupsen/logrus"
	"strconv"
)

func (provider *DockerSwarmProvider) List(ctx context.Context) ([]types.Instance, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	services, err := provider.Client.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: args,
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(services))
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
