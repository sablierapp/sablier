package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	ref, err := p.resolve(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot resolve instance %q: %w", name, err)
	}

	ct, err := p.getContainer(ctx, ref)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	p.l.DebugContext(ctx, "container inspected",
		slog.String("name", ref.name),
		slog.Int("vmid", ref.vmid),
		slog.String("node", ref.node),
		slog.String("status", ct.Status),
	)

	switch ct.Status {
	case "running":
		return sablier.ReadyInstanceState(ref.name, p.desiredReplicas), nil
	case "stopped":
		return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), nil
	default:
		return sablier.UnrecoverableInstanceState(ref.name, fmt.Sprintf("container status %q not handled", ct.Status), p.desiredReplicas), nil
	}
}
