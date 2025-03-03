package docker

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/instance"
	"log/slog"
)

func (p *DockerClassicProvider) InstanceInspect(ctx context.Context, name string) (instance.State, error) {
	spec, err := p.Client.ContainerInspect(ctx, name)
	if err != nil {
		return instance.State{}, fmt.Errorf("cannot inspect container: %w", err)
	}

	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "restarting", "removing":
		return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "running":
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			if spec.State.Health.Status == "healthy" {
				return instance.ReadyInstanceState(name, p.desiredReplicas), nil
			} else if spec.State.Health.Status == "unhealthy" {
				return instance.UnrecoverableInstanceState(name, "container is unhealthy", p.desiredReplicas), nil
			} else {
				return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
			}
		}
		p.l.WarnContext(ctx, "container running without healthcheck, you should define a healthcheck on your container so that Sablier properly detects when the container is ready to handle requests.", slog.String("container", name))
		return instance.ReadyInstanceState(name, p.desiredReplicas), nil
	case "exited":
		if spec.State.ExitCode != 0 {
			return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode), p.desiredReplicas), nil
		}
		return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "dead":
		return instance.UnrecoverableInstanceState(name, "container in \"dead\" state cannot be restarted", p.desiredReplicas), nil
	default:
		return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status), p.desiredReplicas), nil
	}
}
