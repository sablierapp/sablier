package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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

func (p *Provider) getServiceByName(name string, ctx context.Context) (swarm.Service, error) {
	services, _, err := p.Client.ServiceInspectWithRaw(ctx, name, swarm.ServiceInspectOptions{})
	if err != nil {
		return swarm.Service{}, fmt.Errorf("error inspecting service: %w", err)
	}

	p.l.DebugContext(ctx, "service inspected", slog.String("service", name), slog.String("current_replicas", fmt.Sprintf("%d", services.Spec.Mode.Replicated.Replicas)), slog.String("desired_tasks", fmt.Sprintf("%d", services.ServiceStatus.DesiredTasks)), slog.String("running_tasks", fmt.Sprintf("%d", services.ServiceStatus.RunningTasks)))
	return services, nil
}
