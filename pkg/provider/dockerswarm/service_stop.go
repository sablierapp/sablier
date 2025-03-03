package dockerswarm

import "context"

func (p *DockerSwarmProvider) InstanceStop(ctx context.Context, name string) error {
	return p.ServiceUpdateReplicas(ctx, name, 0)
}
