package proxmoxlxc

import "context"

// InstanceName implements sablier.InstanceNameResolver: a container ID (100)
// or node/ID (pve1/100) resolves to the container's hostname, the name
// InstanceInspect reports. A name that cannot be resolved is returned as is.
func (p *Provider) InstanceName(ctx context.Context, name string) string {
	ref, err := p.resolve(ctx, name)
	if err != nil || ref.name == "" {
		return name
	}
	return ref.name
}
