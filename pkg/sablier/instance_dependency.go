package sablier

import (
	"context"
	"fmt"
	"time"
)

// InstanceDependency describes a single dependency of an instance. It must be
// started and reach Condition before the dependent instance can start.
type InstanceDependency struct {
	// Name is the provider-specific identifier of the dependency instance.
	Name string `json:"name"`
	// Condition is the state the dependency must reach. Condition strings
	// follow Docker Compose conventions and map to InstanceStatus values:
	//   service_started               → Starting or Ready
	//   service_healthy               → Ready
	//   service_completed_successfully → Ready
	Condition string `json:"condition"`
}

const (
	// dependencyPollInterval is how often a dependency condition is re-checked.
	dependencyPollInterval = 500 * time.Millisecond
)

// dependencyConditionSatisfied reports whether status satisfies the given
// depends_on condition. Condition strings follow Docker Compose conventions.
func dependencyConditionSatisfied(status InstanceStatus, condition string) bool {
	switch condition {
	case "service_healthy", "service_completed_successfully":
		// Both require the dependency to be fully ready (healthy or exited-0).
		return status == InstanceStatusReady
	default:
		// service_started and unknown conditions: running is enough.
		return status == InstanceStatusStarting || status == InstanceStatusReady
	}
}

// waitForDependencyCondition blocks until the named instance satisfies
// condition (as reported by provider.InstanceInspect), ctx is cancelled, or
// the dependency reports an error status.
func (s *Sablier) waitForDependencyCondition(ctx context.Context, name, condition string) error {
	ticker := time.NewTicker(dependencyPollInterval)
	defer ticker.Stop()

	for {
		info, err := s.provider.InstanceInspect(ctx, name)
		if err != nil {
			return fmt.Errorf("cannot inspect dependency %q: %w", name, err)
		}
		if info.Status == InstanceStatusError {
			return fmt.Errorf("dependency %q is in error state", name)
		}
		if dependencyConditionSatisfied(info.Status, condition) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for dependency %q (condition: %s): %w", name, condition, ctx.Err())
		case <-ticker.C:
		}
	}
}

// startWithDependencies starts all transitive dependencies of name (in
// topological order) before starting name itself. Dependencies that Sablier is
// already starting concurrently (e.g. other members of the same group) are
// detected via pendingStarts and only waited on, never double-started.
func (s *Sablier) startWithDependencies(ctx context.Context, name string) error {
	deps, err := s.provider.InstanceDependencies(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot resolve dependencies of %q: %w", name, err)
	}

	for _, dep := range deps {
		if err := s.startDependency(ctx, dep); err != nil {
			return err
		}
	}

	return s.provider.InstanceStart(ctx, name)
}

// startDependency starts a single dependency and waits for its condition.
// If Sablier is already starting the dependency (it is a group member being
// started concurrently), we wait for that goroutine to complete instead of
// issuing a redundant start call.
func (s *Sablier) startDependency(ctx context.Context, dep InstanceDependency) error {
	s.pendingMu.Lock()
	pending, isManagedBySablier := s.pendingStarts[dep.Name]
	s.pendingMu.Unlock()

	if isManagedBySablier {
		// Dep is a Sablier-managed instance (e.g. a group member started
		// concurrently). Wait for that goroutine to finish rather than racing.
		select {
		case <-pending.done:
			if pending.err != nil {
				return fmt.Errorf("dependency %q start failed: %w", dep.Name, pending.err)
			}
		case <-ctx.Done():
			return fmt.Errorf("context cancelled waiting for dependency %q: %w", dep.Name, ctx.Err())
		}
	} else {
		// Dep is not managed by Sablier for this request. Check first whether
		// the condition is already satisfied (e.g. a previously completed
		// one-shot container) to avoid a gratuitous restart.
		if info, err := s.provider.InstanceInspect(ctx, dep.Name); err == nil &&
			dependencyConditionSatisfied(info.Status, dep.Condition) {
			return nil // condition already met, skip start and condition wait
		}
		if err := s.provider.InstanceStart(ctx, dep.Name); err != nil {
			return fmt.Errorf("cannot start dependency %q: %w", dep.Name, err)
		}
	}

	return s.waitForDependencyCondition(ctx, dep.Name, dep.Condition)
}
