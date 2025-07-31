package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	if service.Spec.Mode.Replicated == nil {
		return sablier.InstanceInfo{}, errors.New("swarm service is not in \"replicated\" mode")
	}

	if service.ServiceStatus.DesiredTasks != service.ServiceStatus.RunningTasks || service.ServiceStatus.DesiredTasks == 0 {
		return sablier.NotReadyInstanceState(service.Spec.Name, 0, p.desiredReplicas), nil
	}

	return sablier.ReadyInstanceState(service.Spec.Name, p.desiredReplicas), nil
}

func (p *Provider) getServiceByName(name string, ctx context.Context) (*swarm.Service, error) {
	opts := swarm.ServiceListOptions{
		Filters: filters.NewArgs(),
		// If set to true, the list will include the swarm.ServiceStatus field to all returned services.
		Status: true,
	}
	opts.Filters.Add("name", name)

	services, err := p.Client.ServiceList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing services: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("service with name %s was not found", name)
	}

	var svc *swarm.Service = nil
	for _, service := range services {
		// Exact match
		if service.Spec.Name == name {
			svc = &service
			break
		}
		if service.ID == name {
			svc = &service
			break
		}
	}

	p.l.DebugContext(ctx, "service inspected", slog.String("service", name),
		slog.Uint64("current_replicas", currentReplicas(svc)),
		slog.Uint64("desired_tasks", desiredReplicas(svc)),
		slog.Uint64("running_tasks", runningReplicas(svc)),
	)
	return svc, nil
}

func currentReplicas(service *swarm.Service) uint64 {
	if service.Spec.Mode.Replicated == nil {
		return 0
	}
	if service.Spec.Mode.Replicated.Replicas == nil {
		return 0
	}
	return *service.Spec.Mode.Replicated.Replicas
}

func desiredReplicas(service *swarm.Service) uint64 {
	if service.ServiceStatus == nil {
		return 0
	}
	if service.ServiceStatus.DesiredTasks == 0 {
		return 0
	}
	return service.ServiceStatus.DesiredTasks
}

func runningReplicas(service *swarm.Service) uint64 {
	if service.ServiceStatus == nil {
		return 0
	}
	if service.ServiceStatus.RunningTasks == 0 {
		return 0
	}
	return service.ServiceStatus.RunningTasks
}
