package docker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types/container"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	pauseInsteadOfStop := false
	containers, inspectErr := p.Client.ContainerInspect(ctx, name)
	if inspectErr == nil {
		pauseInsteadOfStop, _ = strconv.ParseBool(containers.Config.Labels["sablier.pauseOnly"])
	}

	if pauseInsteadOfStop && containers.State.Paused {
		err := p.Client.ContainerUnpause(ctx, name)
		if err != nil {
			return fmt.Errorf("cannot unpause container %s: %w", name, err)
		}
	} else {
		err := p.Client.ContainerStart(ctx, name, container.StartOptions{})
		if err != nil {
			return fmt.Errorf("cannot start container %s: %w", name, err)
		}
	}

	// TODO: InstanceStart should block until the container is ready.

	return nil
}
