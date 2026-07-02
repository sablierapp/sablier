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
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusStarting,
		}
	case container.StateRunning:
		if spec.Container.State.Health != nil {
			// "starting", "healthy" or "unhealthy"
			switch spec.Container.State.Health.Status {
			case container.Healthy:
				info = sablier.InstanceInfo{
					Name:            name,
					CurrentReplicas: 1,
					DesiredReplicas: 1,
					Status:          sablier.InstanceStatusReady,
				}
			case container.Unhealthy:
				info = sablier.InstanceInfo{
					Name:            name,
					CurrentReplicas: 0,
					DesiredReplicas: 1,
					Status:          sablier.InstanceStatusError,
					Message:         "container is unhealthy",
				}
			default: // container.Starting
				info = sablier.InstanceInfo{
					Name:            name,
					CurrentReplicas: 0,
					DesiredReplicas: 1,
					Status:          sablier.InstanceStatusStarting,
				}
			}
		} else {
			p.l.WarnContext(ctx, "container running without healthcheck, you should define a healthcheck on your container so that Sablier properly detects when the container is ready to handle requests.", slog.String("container", name))
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
			}
		}
	case container.StateExited:
		if spec.Container.State.ExitCode != 0 {
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusError,
				Message:         fmt.Sprintf("container exited with code \"%d\"", spec.Container.State.ExitCode),
			}
		} else if p.HonorRestartPolicy && restartsOnSuccess(restartPolicyMode(spec.Container.HostConfig)) {
			// The container exited successfully but its restart policy
			// (always / unless-stopped) means Docker will bring it back up.
			// The exited state is therefore transient.
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStarting,
			}
		} else if p.HonorRestartPolicy {
			// The container exited successfully and its restart policy
			// ("no" / "on-failure") means Docker will not restart it. This is a
			// one-shot / init container (e.g. a database migration) that has
			// completed its job. Report it as completed so that Sablier does not
			// keep restarting it, and so a service_completed_successfully
			// dependency resolves without treating it as a running service. The
			// container is not running, so CurrentReplicas stays 0.
			//
			// Note: Docker normalizes an unset restart policy to "no", so an
			// unset policy is indistinguishable from an explicit "no" and is
			// also reported as completed here.
			// See https://github.com/sablierapp/sablier/issues/952
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusCompleted,
			}
		} else {
			// HonorRestartPolicy is disabled: keep the historical behavior and
			// report a successfully exited container as stopped so that Sablier
			// keeps managing its lifecycle and can start it again on demand.
			// TODO(v2): remove this branch and honor the restart policy by
			// default.
			info = sablier.InstanceInfo{
				Name:            name,
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
			}
		}
	case container.StateDead:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceStatusError,
			Message:         "container in \"dead\" state cannot be restarted",
		}
	default:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: 1,
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

// restartPolicyMode returns the restart policy mode of the container, defaulting
// to RestartPolicyDisabled ("no") when no host configuration is available.
func restartPolicyMode(hc *container.HostConfig) container.RestartPolicyMode {
	if hc == nil {
		return container.RestartPolicyDisabled
	}
	return hc.RestartPolicy.Name
}

// restartsOnSuccess reports whether Docker will restart a container that exited
// with a successful (zero) exit code, given its restart policy.
func restartsOnSuccess(mode container.RestartPolicyMode) bool {
	return mode == container.RestartPolicyAlways || mode == container.RestartPolicyUnlessStopped
}
