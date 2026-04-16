package proxmoxlxc

import (
	"context"
	"log/slog"
	"time"
)

// maxConsecutivePollErrors is the number of consecutive scan failures
// before NotifyInstanceStopped gives up and closes the channel.
const maxConsecutivePollErrors = 5

// NotifyInstanceStopped polls Proxmox for status changes and sends instance
// names to the channel when they transition from running to stopped.
// Proxmox VE does not provide a real-time event stream, so polling is used.
func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	// Track previously seen running containers
	running := make(map[string]bool)

	// Initial scan
	discovered, err := p.scanContainers(ctx)
	if err != nil {
		p.l.ErrorContext(ctx, "initial container scan failed", slog.Any("error", err))
	} else {
		for _, d := range discovered {
			if d.status == "running" {
				running[d.ref.name] = true
			}
		}
	}

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	var consecutiveErrors int

	for {
		select {
		case <-ticker.C:
			discovered, err := p.scanContainers(ctx)
			if err != nil {
				consecutiveErrors++
				p.l.WarnContext(ctx, "container scan failed during polling",
					slog.Any("error", err),
					slog.Int("consecutive_errors", consecutiveErrors),
				)
				if consecutiveErrors >= maxConsecutivePollErrors {
					p.l.ErrorContext(ctx, "too many consecutive poll errors, closing event stream",
						slog.Int("max", maxConsecutivePollErrors),
					)
					close(instance)
					return
				}
				continue
			}

			consecutiveErrors = 0

			currentRunning := make(map[string]bool)
			for _, d := range discovered {
				if d.status == "running" {
					currentRunning[d.ref.name] = true
				}
			}

			// Detect containers that were running but are no longer
			for name := range running {
				if !currentRunning[name] {
					p.l.DebugContext(ctx, "container stopped detected", slog.String("name", name))
					instance <- name
				}
			}

			running = currentRunning
		case <-ctx.Done():
			close(instance)
			return
		}
	}
}
