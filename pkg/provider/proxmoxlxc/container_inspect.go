package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"

	proxmox "github.com/luthermonson/go-proxmox"
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
		// A running container clears any recorded start failure.
		p.mu.Lock()
		delete(p.failedStarts, name)
		p.mu.Unlock()

		// Check if the container's network interfaces have an IP address assigned.
		// Proxmox reports "running" as soon as the LXC starts, but services inside
		// may not be ready yet. Checking for a non-loopback interface with an IP
		// is a lightweight readiness signal.
		ifaces, err := ct.Interfaces(ctx)
		if err != nil {
			p.l.WarnContext(ctx, "cannot check container interfaces, assuming ready",
				slog.String("name", ref.name),
				slog.Any("error", err),
			)
			return sablier.ReadyInstanceState(ref.name, p.desiredReplicas), nil
		}

		if !hasNonLoopbackIP(ifaces) {
			p.l.DebugContext(ctx, "container running but no network interface has an IP yet",
				slog.String("name", ref.name),
			)
			return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), nil
		}

		return sablier.ReadyInstanceState(ref.name, p.desiredReplicas), nil
	case "stopped":
		// If a previous start attempt failed, report as unrecoverable so the
		// UI shows the error instead of retrying indefinitely.
		p.mu.RLock()
		failMsg, failed := p.failedStarts[name]
		p.mu.RUnlock()
		if failed {
			return sablier.UnrecoverableInstanceState(ref.name, failMsg, p.desiredReplicas), nil
		}
		return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), nil
	default:
		return sablier.UnrecoverableInstanceState(ref.name, fmt.Sprintf("container status %q not handled", ct.Status), p.desiredReplicas), nil
	}
}

// hasNonLoopbackIP returns true if any non-loopback interface has an IPv4 or IPv6 address.
func hasNonLoopbackIP(ifaces proxmox.ContainerInterfaces) bool {
	for _, iface := range ifaces {
		if iface.Name == "lo" {
			continue
		}
		if iface.Inet != "" || iface.Inet6 != "" {
			return true
		}
	}
	return false
}
