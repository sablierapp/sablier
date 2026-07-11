package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	service, err := p.getServiceByName(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	if service.Spec.Mode.Replicated == nil {
		return sablier.InstanceInfo{}, errors.New("swarm service is not in \"replicated\" mode")
	}

	sc := sablier.ScaleConfigFromLabels(service.Spec.Labels)
	desired := sc.Active.Replicas

	var info sablier.InstanceInfo
	if service.ServiceStatus.DesiredTasks == 0 {
		info = sablier.InstanceInfo{
			Name:            service.Spec.Name,
			CurrentReplicas: 0,
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusStopped,
		}
	} else if service.ServiceStatus.DesiredTasks != service.ServiceStatus.RunningTasks {
		info = sablier.InstanceInfo{
			Name:            service.Spec.Name,
			CurrentReplicas: int32(service.ServiceStatus.RunningTasks),
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusStarting,
		}
	} else {
		info = sablier.InstanceInfo{
			Name:            service.Spec.Name,
			CurrentReplicas: int32(service.ServiceStatus.RunningTasks),
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusReady,
		}
	}

	labels := service.Spec.Labels
	sablier.PopulateEnabledAndGroup(&info, labels)

	var image string
	if service.Spec.TaskTemplate.ContainerSpec != nil {
		image = service.Spec.TaskTemplate.ContainerSpec.Image
	}
	info.Provider = sablier.ProviderSwarm
	info.Swarm = &sablier.SwarmServiceInfo{
		ID:     service.ID,
		Image:  image,
		Labels: labels,
	}

	return info, nil
}

func (p *Provider) getServiceByName(ctx context.Context, name string) (*swarm.Service, error) {
	filters := client.Filters{}
	filters.Add("name", name)

	opts := client.ServiceListOptions{
		Filters: filters,
		// If set to true, the list will include the swarm.ServiceStatus field to all returned services.
		Status: true,
	}

	services, err := p.Client.ServiceList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing services: %w", err)
	}

	// The "name" filter is a substring match, so the list can be non-empty
	// while containing no service actually named `name`; only an exact name or
	// ID match counts.
	var svc *swarm.Service
	for _, service := range services.Items {
		if service.Spec.Name == name || service.ID == name {
			svc = &service
			break
		}
	}
	if svc == nil {
		return nil, fmt.Errorf("service with name %s was not found", name)
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
