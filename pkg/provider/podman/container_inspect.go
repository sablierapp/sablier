package podman

import (
	"context"
	"fmt"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	info, err := p.Provider.InstanceInspect(ctx, name)
	if err != nil {
		return info, err
	}

	if info.Docker == nil {
		return sablier.InstanceInfo{}, fmt.Errorf("podman: docker provider did not populate Docker field for %q", name)
	}

	info.Provider = sablier.ProviderPodman
	info.Podman = &sablier.PodmanContainerInfo{
		ID:     info.Docker.ID,
		Image:  info.Docker.Image,
		Labels: info.Docker.Labels,
	}
	info.Docker = nil

	return info, nil
}
