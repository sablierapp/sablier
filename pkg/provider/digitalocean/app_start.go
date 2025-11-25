package digitalocean

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/digitalocean/godo"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "starting app", "name", name)

	appID := name

	// Get current app to check deployment status
	app, _, err := p.Client.Apps.Get(ctx, appID)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot get app", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot get app %s: %w", name, err)
	}

	// Check if app components need to be scaled up
	needsUpdate := false
	updateRequest := &godo.AppUpdateRequest{
		Spec: app.Spec,
	}

	// Scale up services
	for i, service := range updateRequest.Spec.Services {
		if service.InstanceCount == 0 {
			updateRequest.Spec.Services[i].InstanceCount = 1
			needsUpdate = true
		}
	}

	// Scale up workers
	for i, worker := range updateRequest.Spec.Workers {
		if worker.InstanceCount == 0 {
			updateRequest.Spec.Workers[i].InstanceCount = 1
			needsUpdate = true
		}
	}

	if !needsUpdate {
		p.l.DebugContext(ctx, "app already running", slog.String("name", name))
		return nil
	}

	// Update the app to scale it up
	_, _, err = p.Client.Apps.Update(ctx, appID, updateRequest)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start app", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start app %s: %w", name, err)
	}

	p.l.InfoContext(ctx, "app started", slog.String("name", name))
	return nil
}
