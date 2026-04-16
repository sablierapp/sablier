package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	ref, err := p.resolve(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot resolve instance %q: %w", name, err)
	}

	// Check if there is a pending start task for this instance.
	p.mu.Lock()
	pt, hasPending := p.pendingTasks[name]
	p.mu.Unlock()

	if hasPending {
		info, done := p.checkPendingTask(ctx, name, ref, pt)
		if !done {
			return info, nil
		}
		// If the failed task TTL expired, attempt a fresh start so the caller
		// (Sablier core) doesn't need to call InstanceStart — its session still
		// exists with not-ready status, so only InstanceInspect will be called.
		if !pt.failedAt.IsZero() {
			if err := p.InstanceStart(ctx, name); err != nil {
				p.l.WarnContext(ctx, "retry start after failed task expiry failed",
					slog.String("name", ref.name),
					slog.Any("error", err),
				)
			} else {
				return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), nil
			}
		}
		// Task completed successfully — fall through to normal status check.
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
		return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), nil
	default:
		return sablier.UnrecoverableInstanceState(ref.name, fmt.Sprintf("container status %q not handled", ct.Status), p.desiredReplicas), nil
	}
}

// checkPendingTask polls the Proxmox task status and returns the appropriate state.
// It returns (info, done) where done=true means the task completed (or the failure
// TTL expired) and the caller should fall through to the normal container status check.
func (p *Provider) checkPendingTask(ctx context.Context, name string, ref containerRef, pt *pendingTask) (sablier.InstanceInfo, bool) {
	if err := pt.task.Ping(ctx); err != nil {
		p.l.WarnContext(ctx, "cannot ping start task, assuming in progress",
			slog.String("name", ref.name),
			slog.Any("error", err),
		)
		return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), false
	}

	if pt.task.IsRunning {
		p.l.DebugContext(ctx, "start task still running",
			slog.String("name", ref.name),
			slog.String("upid", string(pt.task.UPID)),
		)
		return sablier.NotReadyInstanceState(ref.name, 0, p.desiredReplicas), false
	}

	if pt.task.IsFailed {
		// Record when we first observed the failure. task.EndTime from the API
		// is preferred, but copier.Copy in go-proxmox may zero it out (json:"-"),
		// so failedAt is a reliable fallback.
		if pt.failedAt.IsZero() {
			if !pt.task.EndTime.IsZero() {
				pt.failedAt = pt.task.EndTime
			} else {
				pt.failedAt = time.Now()
			}
		}

		// Keep returning Unrecoverable until failedTaskTTL after the task finished.
		// This gives the user time to see the error while polling, and automatically
		// clears the entry so a fresh InstanceStart can be attempted later.
		if time.Since(pt.failedAt) > failedTaskTTL {
			p.mu.Lock()
			delete(p.pendingTasks, name)
			p.mu.Unlock()
			p.l.InfoContext(ctx, "cleared expired failed start task",
				slog.String("name", ref.name),
				slog.Duration("since_failed", time.Since(pt.failedAt)),
			)
			return sablier.InstanceInfo{}, true
		}
		msg := fmt.Sprintf("start task failed for container %q (VMID %d): %s", ref.name, ref.vmid, pt.task.ExitStatus)
		p.l.WarnContext(ctx, msg)
		return sablier.UnrecoverableInstanceState(ref.name, msg, p.desiredReplicas), false
	}

	// Task completed successfully — remove from pending.
	p.mu.Lock()
	delete(p.pendingTasks, name)
	p.mu.Unlock()

	p.l.DebugContext(ctx, "start task completed successfully",
		slog.String("name", ref.name),
		slog.String("upid", string(pt.task.UPID)),
	)
	// done=true: task succeeded, fall through to normal container status check.
	return sablier.InstanceInfo{}, true
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
