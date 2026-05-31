package docker

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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
func (p *Provider) startDependencies(ctx context.Context, labels map[string]string, tracker *startTracker) error {
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

		// Break dependency cycles: if the dependency is already being started
		// further up the recursion stack, starting or waiting on it here would
		// deadlock until the context times out. Skip it and let the in-progress
		// start higher up resolve it.
		if _, inProgress := tracker.inProgress[name]; inProgress {
			p.l.WarnContext(ctx, "dependency cycle detected, skipping depends_on dependency",
				slog.String("dependency", name),
				slog.String("condition", dep.Condition),
			)
			continue
		}

		p.l.DebugContext(ctx, "starting depends_on dependency",
			slog.String("dependency", name),
			slog.String("condition", dep.Condition),
		)

		if err = p.instanceStart(ctx, name, tracker); err != nil {
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
//
// Multiple containers can match the same project+service labels (scaled
// services, or leftover containers from a previous run). The Docker API does
// not guarantee any ordering, so the selection is made deterministic: a running
// container is preferred, otherwise the lexicographically smallest name is
// chosen.
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

	var names, running []string
	for _, c := range containers.Items {
		if len(c.Names) == 0 {
			continue
		}
		name := strings.TrimPrefix(c.Names[0], "/")
		names = append(names, name)
		if c.State == container.StateRunning {
			running = append(running, name)
		}
	}

	if len(running) > 0 {
		sort.Strings(running)
		return running[0], nil
	}
	if len(names) == 0 {
		return "", nil
	}
	sort.Strings(names)
	return names[0], nil
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
		// A service_healthy dependency on a container without a healthcheck can
		// never be satisfied. Fail fast with a clear error instead of looping
		// until the context deadline is exceeded. The container must be running
		// before its health can be evaluated, so only fail once it is up.
		if state.Running && state.Health == nil {
			return false, fmt.Errorf("dependency %s has no healthcheck configured but condition %q requires one", name, conditionServiceHealthy)
		}
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
