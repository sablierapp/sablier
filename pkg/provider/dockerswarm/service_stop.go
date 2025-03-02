package dockerswarm

import "context"

func (p *DockerSwarmProvider) Stop(ctx context.Context, name string) error {
	return p.scale(ctx, name, 0)
}
