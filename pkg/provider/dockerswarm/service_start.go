package dockerswarm

import "context"

func (p *DockerSwarmProvider) Start(ctx context.Context, name string) error {
	return p.scale(ctx, name, uint64(p.desiredReplicas))
}
