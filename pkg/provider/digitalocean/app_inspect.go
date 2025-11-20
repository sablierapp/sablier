package digitalocean

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	appID := name

	app, _, err := p.Client.Apps.Get(ctx, appID)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot inspect app: %w", err)
	}

	p.l.DebugContext(ctx, "app inspected",
		slog.String("app", name),
		slog.String("phase", string(app.ActiveDeployment.Phase)),
	)

	// Calculate total current and desired instances
	var currentReplicas, desiredReplicas int32

	// Check services
	for _, service := range app.Spec.Services {
		desiredReplicas += int32(service.InstanceCount)
	}

	// Check workers
	for _, worker := range app.Spec.Workers {
		desiredReplicas += int32(worker.InstanceCount)
	}

	// Count running instances from active deployment
	if app.ActiveDeployment != nil {
		// Deployment phases: "PENDING_BUILD", "BUILDING", "PENDING_DEPLOY", "DEPLOYING", "ACTIVE", "SUPERSEDED", "ERROR", "CANCELED"
		switch app.ActiveDeployment.Phase {
		case "ACTIVE":
			// Count actual running instances
			for _, service := range app.Spec.Services {
				currentReplicas += int32(service.InstanceCount)
			}
			for _, worker := range app.Spec.Workers {
				currentReplicas += int32(worker.InstanceCount)
			}

			if currentReplicas > 0 {
				return sablier.ReadyInstanceState(name, desiredReplicas), nil
			}
			return sablier.NotReadyInstanceState(name, currentReplicas, desiredReplicas), nil

		case "BUILDING", "PENDING_BUILD", "DEPLOYING", "PENDING_DEPLOY":
			return sablier.NotReadyInstanceState(name, currentReplicas, desiredReplicas), nil

		case "ERROR", "CANCELED":
			return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("deployment in %s state", app.ActiveDeployment.Phase), desiredReplicas), nil

		case "SUPERSEDED":
			return sablier.NotReadyInstanceState(name, currentReplicas, desiredReplicas), nil

		default:
			return sablier.UnrecoverableInstanceState(name, fmt.Sprintf("deployment phase \"%s\" not handled", app.ActiveDeployment.Phase), desiredReplicas), nil
		}
	}

	return sablier.NotReadyInstanceState(name, 0, desiredReplicas), nil
}
