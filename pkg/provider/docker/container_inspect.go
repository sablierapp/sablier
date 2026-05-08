package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot inspect container: %w", err)
	}

	p.l.DebugContext(ctx, "container inspected", slog.String("container", name), slog.String("status", string(spec.Container.State.Status)), slog.String("health", healthStatus(spec.Container.State.Health)))

	var info sablier.InstanceInfo
	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.Container.State.Status {
	case container.StateCreated, container.StatePaused, container.StateRestarting, container.StateRemoving:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: p.desiredReplicas,
			Status:          sablier.InstanceStatusStarting,
		}
	case container.StateRunning:
		if spec.Container.State.Health != nil {
			// "starting", "healthy" or "unhealthy"
			switch spec.Container.State.Health.Status {
			case container.Healthy:
				info = sablier.InstanceInfo{
					Name:            name,
					CurrentReplicas: p.desiredReplicas,
					DesiredReplicas: p.desiredReplicas,
					Status:          sablier.InstanceStatusReady,
				}
			case container.Unhealthy:
				info = sablier.InstanceInfo{
					Name:            name,
					CurrentReplicas: 0,
					DesiredReplicas: p.desiredReplicas,
					Status:          sablier.InstanceStatusError,
					Message:         "container is unhealthy",
				}
			default: // container.Starting
				info = sablier.InstanceInfo{
					Name:            name,
					CurrentReplicas: 0,
					DesiredReplicas: p.desiredReplicas,
					Status:          sablier.InstanceStatusStarting,
				}
			}
		} else {
			p.l.WarnContext(ctx, "container running without healthcheck, you should define a healthcheck on your container so that Sablier properly detects when the container is ready to handle requests.", slog.String("container", name))
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: p.desiredReplicas,
				DesiredReplicas: p.desiredReplicas,
				Status:          sablier.InstanceStatusReady,
			}
		}
	case container.StateExited:
		if spec.Container.State.ExitCode != 0 {
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 0,
				DesiredReplicas: p.desiredReplicas,
				Status:          sablier.InstanceStatusError,
				Message:         fmt.Sprintf("container exited with code \"%d\"", spec.Container.State.ExitCode),
			}
		} else {
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 0,
				DesiredReplicas: p.desiredReplicas,
				Status:          sablier.InstanceStatusStopped,
			}
		}
	case container.StateDead:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: p.desiredReplicas,
			Status:          sablier.InstanceStatusError,
			Message:         "container in \"dead\" state cannot be restarted",
		}
	default:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: p.desiredReplicas,
			Status:          sablier.InstanceStatusError,
			Message:         fmt.Sprintf("container status \"%s\" not handled", spec.Container.State.Status),
		}
	}

	labels := spec.Container.Config.Labels
	sablier.PopulateEnabledAndGroup(&info, labels)

	info.Provider = sablier.ProviderDocker
	info.Docker = &sablier.DockerContainerInfo{
		ID:     spec.Container.ID,
		Image:  spec.Container.Config.Image,
		Labels: labels,
	}

	return info, nil
}

func healthStatus(health *container.Health) string {
	if health == nil {
		return "no healthcheck defined"
	}

	return string(health.Status)
}
