package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	// composeProjectLabel identifies the Docker Compose project a container belongs to.
	composeProjectLabel = "com.docker.compose.project"
	// composeServiceLabel identifies the Docker Compose service a container implements.
	composeServiceLabel = "com.docker.compose.service"
	// composeDependsOnLabel stores the service dependencies of a container.
	// Its value is a comma-separated list of "service:condition:restart" entries,
	// e.g. "db:service_healthy:false,migration:service_completed_successfully:false".
	composeDependsOnLabel = "com.docker.compose.depends_on"
)

// Docker Compose depends_on conditions.
const (
	conditionServiceStarted               = "service_started"
	conditionServiceHealthy               = "service_healthy"
	conditionServiceCompletedSuccessfully = "service_completed_successfully"
	conditionServiceRunningOrHealthy      = "service_running_or_healthy"
)

// dependencyPollInterval is how often the dependency conditions are re-checked
// while waiting for them to be satisfied.
const dependencyPollInterval = 500 * time.Millisecond

// composeDependency represents a single Docker Compose depends_on edge.
type composeDependency struct {
	Service   string
	Condition string
}

// parseComposeDependsOn parses the value of the com.docker.compose.depends_on
// label into a list of dependencies. The expected format is a comma-separated
// list of "service:condition:restart" entries. Malformed entries are skipped.
func parseComposeDependsOn(label string) []composeDependency {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil
	}

	var deps []composeDependency
	for _, entry := range strings.Split(label, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		deps = append(deps, composeDependency{
			Service:   parts[0],
			Condition: parts[1],
		})
	}
	return deps
}

// startDependencies resolves the Docker Compose depends_on dependencies declared
// in the given container labels, starting each dependency (recursively) and
// waiting until its declared condition is satisfied before returning.
//
// Dependencies that cannot be resolved to a running-or-creatable container (for
// example external services not managed by this Compose project) are skipped.
func (p *Provider) startDependencies(ctx context.Context, labels map[string]string, started map[string]struct{}) error {
	deps := parseComposeDependsOn(labels[composeDependsOnLabel])
	if len(deps) == 0 {
		return nil
	}

	project := labels[composeProjectLabel]

	for _, dep := range deps {
		name, err := p.findComposeContainer(ctx, project, dep.Service)
		if err != nil {
			return err
		}
		if name == "" {
			p.l.WarnContext(ctx, "skipping depends_on dependency, no container found",
				slog.String("service", dep.Service),
				slog.String("project", project),
			)
			continue
		}

		p.l.DebugContext(ctx, "starting depends_on dependency",
			slog.String("dependency", name),
			slog.String("condition", dep.Condition),
		)

		if err = p.instanceStart(ctx, name, started); err != nil {
			return fmt.Errorf("cannot start dependency %s: %w", name, err)
		}

		if err = p.waitForDependencyCondition(ctx, name, dep.Condition); err != nil {
			return fmt.Errorf("dependency %s did not satisfy condition %q: %w", name, dep.Condition, err)
		}
	}

	return nil
}

// findComposeContainer returns the container name for the given Compose project
// and service. It returns an empty name (and no error) when no matching
// container exists.
func (p *Provider) findComposeContainer(ctx context.Context, project, service string) (string, error) {
	filters := client.Filters{}
	if project != "" {
		filters.Add("label", fmt.Sprintf("%s=%s", composeProjectLabel, project))
	}
	filters.Add("label", fmt.Sprintf("%s=%s", composeServiceLabel, service))

	containers, err := p.Client.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return "", fmt.Errorf("cannot list containers for dependency %s: %w", service, err)
	}

	if len(containers.Items) == 0 {
		return "", nil
	}

	return strings.TrimPrefix(containers.Items[0].Names[0], "/"), nil
}

// waitForDependencyCondition blocks until the given container satisfies the
// requested Docker Compose depends_on condition, the context is cancelled, or
// the dependency fails (e.g. a service_completed_successfully dependency that
// exits with a non-zero code).
func (p *Provider) waitForDependencyCondition(ctx context.Context, name, condition string) error {
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
		case <-time.After(dependencyPollInterval):
		}
	}
}

// checkDependencyCondition reports whether the container currently satisfies the
// requested condition. It returns an error when the condition can never be
// satisfied (e.g. the container exited unsuccessfully for a
// service_completed_successfully dependency).
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
		return true, nil
	case conditionServiceHealthy:
		return isHealthy(state, false), nil
	case conditionServiceRunningOrHealthy:
		return isHealthy(state, true), nil
	case conditionServiceStarted, "":
		// Default to service_started semantics for unknown/empty conditions.
		return state.Running, nil
	default:
		p.l.WarnContext(ctx, "unsupported depends_on condition, falling back to service_started",
			slog.String("dependency", name),
			slog.String("condition", condition),
		)
		return state.Running, nil
	}
}

// isHealthy reports whether the container is healthy. When fallbackRunning is
// true and the container has no healthcheck, it falls back to reporting whether
// the container is running.
func isHealthy(state *container.State, fallbackRunning bool) bool {
	if state == nil || !state.Running {
		return false
	}
	if state.Health == nil {
		return fallbackRunning
	}
	return state.Health.Status == container.Healthy
}
