package dockerswarm

import "context"

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	return p.ServiceUpdateReplicas(ctx, name, 0)
}
