package digitalocean

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/digitalocean/godo"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping app", slog.String("name", name))

	appID := name

	// Get current app
	app, _, err := p.Client.Apps.Get(ctx, appID)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot get app", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot get app %s: %w", name, err)
	}

	// Scale down all components to 0
	updateRequest := &godo.AppUpdateRequest{
		Spec: app.Spec,
	}

	// Scale down services
	for i := range updateRequest.Spec.Services {
		updateRequest.Spec.Services[i].InstanceCount = 0
	}

	// Scale down workers
	for i := range updateRequest.Spec.Workers {
		updateRequest.Spec.Workers[i].InstanceCount = 0
	}

	// Update the app to scale it down
	_, _, err = p.Client.Apps.Update(ctx, appID, updateRequest)
	if err != nil {
		p.l.ErrorContext(ctx, "cannot stop app", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot stop app %s: %w", name, err)
	}

	p.l.InfoContext(ctx, "app stopped", slog.String("name", name))
	return nil
}
