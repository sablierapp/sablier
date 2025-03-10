package dockerswarm

import "context"

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	return p.ServiceUpdateReplicas(ctx, name, uint64(p.desiredReplicas))
}
