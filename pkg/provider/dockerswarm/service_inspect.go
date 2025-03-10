package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	foundName := p.getInstanceName(name, *service)

	if service.Spec.Mode.Replicated == nil {
		return sablier.InstanceInfo{}, errors.New("swarm service is not in \"replicated\" mode")
	}

	if service.ServiceStatus.DesiredTasks != service.ServiceStatus.RunningTasks || service.ServiceStatus.DesiredTasks == 0 {
		return sablier.NotReadyInstanceState(foundName, 0, p.desiredReplicas), nil
	}

	return sablier.ReadyInstanceState(foundName, p.desiredReplicas), nil
}

func (p *Provider) getServiceByName(name string, ctx context.Context) (*swarm.Service, error) {
	opts := types.ServiceListOptions{
		Filters: filters.NewArgs(),
		Status:  true,
	}
	opts.Filters.Add("name", name)

	services, err := p.Client.ServiceList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing services: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("service with name %s was not found", name)
	}

	for _, service := range services {
		// Exact match
		if service.Spec.Name == name {
			return &service, nil
		}
		if service.ID == name {
			return &service, nil
		}
	}

	return nil, fmt.Errorf("service %s was not found because it did not match exactly or on suffix", name)
}
