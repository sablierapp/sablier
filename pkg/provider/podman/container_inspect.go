package podman

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	spec, err := containers.Inspect(p.conn, name, nil)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot inspect container: %w", err)
	}
	p.l.DebugContext(ctx, "container inspected", slog.String("container", name), slog.String("status", spec.State.Status), slog.Any("health", spec.State.Health))

	status, err := define.StringToContainerStatus(spec.State.Status)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot convert container status: %w", err)
	}

	switch status {
	case define.ContainerStateConfigured, define.ContainerStateCreated, define.ContainerStatePaused, define.ContainerStateRemoving:
		return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case define.ContainerStateRunning:
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			switch spec.State.Health.Status {
			case define.HealthCheckHealthy:
				return sablier.ReadyInstanceState(name, p.desiredReplicas), nil
			case define.HealthCheckUnhealthy:
				return sablier.UnrecoverableInstanceState(name, "container is unhealthy", p.desiredReplicas), nil
			default:
				return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
			}
		}
		p.l.WarnContext(ctx, "container running without healthcheck, you should define a healthcheck on your container so that Sablier properly detects when the container is ready to handle requests.", slog.String("container", name))
		return sablier.ReadyInstanceState(name, p.desiredReplicas), nil
	case define.ContainerStateExited, define.ContainerStateStopped, define.ContainerStateStopping:
		if spec.State.ExitCode != 0 {
			return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode), p.desiredReplicas), nil
		}
		return sablier.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	default:
		return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status), p.desiredReplicas), nil
	}
}
