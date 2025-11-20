package digitalocean

import (
	"context"
	"log/slog"
	"time"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	// Digital Ocean doesn't provide a native event stream API like Docker
	// We need to poll for changes in app deployments
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Keep track of last known state
	lastState := make(map[string]int32)

	for {
		select {
		case <-ticker.C:
			apps, _, err := p.Client.Apps.List(ctx, nil)
			if err != nil {
				p.l.ErrorContext(ctx, "failed to list apps", slog.Any("error", err))
				continue
			}

			for _, app := range apps {
				// Check if this app has sablier enabled
				enabled := false
				for _, env := range app.Spec.Envs {
					if env.Key == "SABLIER_ENABLE" && env.Value == "true" {
						enabled = true
						break
					}
				}

				if !enabled {
					continue
				}

				// Calculate current instance count
				var currentCount int32
				for _, service := range app.Spec.Services {
					currentCount += int32(service.InstanceCount)
				}
				for _, worker := range app.Spec.Workers {
					currentCount += int32(worker.InstanceCount)
				}

				// Check if app was stopped (went from running to 0 instances)
				if lastCount, exists := lastState[app.ID]; exists {
					if lastCount > 0 && currentCount == 0 {
						p.l.DebugContext(ctx, "app stopped detected", slog.String("app_id", app.ID))
						instance <- app.ID
					}
				}

				lastState[app.ID] = currentCount
			}

		case <-ctx.Done():
			close(instance)
			return
		}
	}
}
