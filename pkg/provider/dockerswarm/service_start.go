package dockerswarm

import "context"

func (p *DockerSwarmProvider) InstanceStart(ctx context.Context, name string) error {
	return p.ServiceUpdateReplicas(ctx, name, uint64(p.desiredReplicas))
}
