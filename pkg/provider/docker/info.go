package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/sablierapp/sablier/pkg/sablier"
	"time"
)

func (d *DockerProvider) Info(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	info, _, err := d.InfoWithSpec(ctx, name)
	return info, err
}

func (d *DockerProvider) InfoWithSpec(ctx context.Context, name string) (sablier.InstanceInfo, types.ContainerJSON, error) {
	spec, err := d.Client.ContainerInspect(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, types.ContainerJSON{}, err
	}

	// String representation of the container state.
	// Can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "exited", "dead":
		return sablier.InstanceInfo{
			Name:            FormatName(spec.Name),
			CurrentReplicas: 0,
			DesiredReplicas: 1,
			Status:          sablier.InstanceDown,
			StartedAt:       time.Time{},
		}, spec, nil
	case "running":
		startedAt, err := time.Parse(time.RFC3339Nano, spec.State.StartedAt)
		if err != nil {
			return sablier.InstanceInfo{}, types.ContainerJSON{}, err
		}

		if spec.State.Health != nil && spec.State.Health.Status != "healthy" {
			return sablier.InstanceInfo{
				Name:            FormatName(spec.Name),
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStarting,
				StartedAt:       startedAt,
			}, spec, nil
		}

		return sablier.InstanceInfo{
			Name:            FormatName(spec.Name),
			CurrentReplicas: 1,
			DesiredReplicas: 1,
			Status:          sablier.InstanceReady,
			StartedAt:       startedAt,
		}, spec, nil
	default:
		return sablier.InstanceInfo{}, types.ContainerJSON{}, fmt.Errorf("unknown container status: %s", spec.State.Status)
	}
}
