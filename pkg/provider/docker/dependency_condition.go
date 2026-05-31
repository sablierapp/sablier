package docker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// dependencyPollInterval is how often depends_on conditions are re-checked while
// waiting for them to be satisfied.
const dependencyPollInterval = 500 * time.Millisecond

// waitForDependencyCondition blocks until name satisfies condition, ctx is
// cancelled, or the dependency fails irrecoverably (e.g. a
// service_completed_successfully dependency that exits non-zero).
func (p *Provider) waitForDependencyCondition(ctx context.Context, name, condition string) error {
	ticker := time.NewTicker(dependencyPollInterval)
	defer ticker.Stop()

	for {
		satisfied, err := p.checkDependencyCondition(ctx, name, condition)
		if err != nil {
			return err
		}
		if satisfied {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// checkDependencyCondition reports whether name currently satisfies condition.
// It returns an error when the condition can never be satisfied.
func (p *Provider) checkDependencyCondition(ctx context.Context, name, condition string) (bool, error) {
	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return false, fmt.Errorf("cannot inspect dependency %s: %w", name, err)
	}
	state := spec.Container.State

	switch condition {
	case conditionServiceCompletedSuccessfully:
		if state.Status != container.StateExited {
			return false, nil
		}
		if state.ExitCode != 0 {
			return false, fmt.Errorf("container exited with code %d", state.ExitCode)
		}
		// A container with always/unless-stopped restart policy is only
		// transiently exited; Docker will restart it, so it has not
		// "completed successfully" yet.
		if restartsOnSuccess(restartPolicyMode(spec.Container.HostConfig)) {
			return false, nil
		}
		return true, nil
	case conditionServiceHealthy:
		// A running container without a healthcheck can never become healthy:
		// fail fast instead of polling until the context deadline.
		if state.Running && state.Health == nil {
			return false, fmt.Errorf("dependency %s has no healthcheck configured but condition %q requires one", name, conditionServiceHealthy)
		}
		return isHealthy(state, false), nil
	case conditionServiceRunningOrHealthy:
		return isHealthy(state, true), nil
	case conditionServiceStarted, "":
		return state.Running, nil
	default:
		p.l.WarnContext(ctx, "unsupported depends_on condition, falling back to service_started",
			slog.String("dependency", name),
			slog.String("condition", condition),
		)
		return state.Running, nil
	}
}

// isHealthy reports whether the container is healthy. When the container has no
// healthcheck, it falls back to fallbackRunning.
func isHealthy(state *container.State, fallbackRunning bool) bool {
	if state == nil || !state.Running {
		return false
	}
	if state.Health == nil {
		return fallbackRunning
	}
	return state.Health.Status == container.Healthy
}
