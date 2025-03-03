package dockerswarm

import "context"

func (p *DockerSwarmProvider) Stop(ctx context.Context, name string) error {
	return p.ServiceUpdateReplicas(ctx, name, 0)
}
