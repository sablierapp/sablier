package proxmoxlxc

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// maxConsecutivePollErrors is the number of consecutive scan failures
// before InstanceEvents gives up and reports a terminal error.
const maxConsecutivePollErrors = 5

// InstanceEvents polls Proxmox for status changes and emits events when
// instances transition from running to stopped (or vice versa). Proxmox VE
// does not provide a real-time event stream, so polling is used.
func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	eventsC := make(chan sablier.InstanceInfo)
	errC := make(chan error, 1)

	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	wantStarted := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted)

	go func() {
		defer close(eventsC)
		defer close(errC)

		if !wantStopped && !wantStarted {
			<-ctx.Done()
			return
		}

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
						errC <- err
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
				if wantStopped {
					for name := range running {
						if !currentRunning[name] {
							p.l.DebugContext(ctx, "container stopped detected", slog.String("name", name))
							select {
							case eventsC <- sablier.InstanceInfo{
								Name:            name,
								CurrentReplicas: 0,
								DesiredReplicas: p.desiredReplicas,
								Status:          sablier.InstanceStatusStopped,
							}:
							case <-ctx.Done():
								return
							}
						}
					}
				}

				// Detect containers that were not running but are now
				if wantStarted {
					for name := range currentRunning {
						if !running[name] {
							p.l.DebugContext(ctx, "container started detected", slog.String("name", name))
							select {
							case eventsC <- sablier.InstanceInfo{
								Name:            name,
								CurrentReplicas: p.desiredReplicas,
								DesiredReplicas: p.desiredReplicas,
								Status:          sablier.InstanceStatusStarting,
							}:
							case <-ctx.Done():
								return
							}
						}
					}
				}

				running = currentRunning
			case <-ctx.Done():
				return
			}
		}
	}()

	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}
