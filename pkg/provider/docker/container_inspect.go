package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	spec, err := p.Client.ContainerInspect(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot inspect container: %w", err)
	}

	p.l.DebugContext(ctx, "container inspected", slog.String("container", name), slog.String("status", spec.State.Status), slog.String("health", healthStatus(spec.State.Health)))

	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "restarting", "removing":
		return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "running":
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			switch spec.State.Health.Status {
			case "healthy":
				return sablier.ReadyInstanceState(name, p.desiredReplicas), nil
			case "unhealthy":
				return sablier.UnrecoverableInstanceState(name, "container is unhealthy", p.desiredReplicas), nil
			default:
				return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
			}
		}
		p.l.WarnContext(ctx, "container running without healthcheck, you should define a healthcheck on your container so that Sablier properly detects when the container is ready to handle requests.", slog.String("container", name))
		return sablier.ReadyInstanceState(name, p.desiredReplicas), nil
	case "exited":
		if spec.State.ExitCode != 0 {
			return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode), p.desiredReplicas), nil
		}
		return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "dead":
		return sablier.UnrecoverableInstanceState(name, "container in \"dead\" state cannot be restarted", p.desiredReplicas), nil
	default:
		return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status), p.desiredReplicas), nil
	}
}

func healthStatus(health *container.Health) string {
	if health == nil {
		return "no healthcheck defined"
	}

	return health.Status
}
