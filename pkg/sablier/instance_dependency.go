package sablier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// InstanceDependency describes a single direct dependency of an instance. It must
// be started and reach Condition before the dependent instance can start.
type InstanceDependency struct {
	// Name is the provider-specific identifier of the dependency instance.
	Name string `json:"name"`
	// Condition is the state the dependency must reach. Condition strings
	// follow Docker Compose conventions and map to InstanceStatus values:
	//   service_started                → Starting, Ready or Completed
	//   service_healthy                → Ready
	//   service_running_or_healthy     → Starting or Ready
	//   service_completed_successfully → Completed
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
	case "service_healthy":
		// The dependency must be running and pass its health check.
		return status == InstanceStatusReady
	case "service_completed_successfully":
		// The dependency must have exited successfully (a completed one-shot).
		return status == InstanceStatusCompleted
	case "service_running_or_healthy":
		// The dependency must be running (or healthy if it has a health check).
		return status == InstanceStatusStarting || status == InstanceStatusReady
	default:
		// service_started and unknown conditions: the dependency only needs to
		// have started. A one-shot that already ran (Completed) also qualifies.
		return status == InstanceStatusStarting || status == InstanceStatusReady || status == InstanceStatusCompleted
	}
}

// startWithDependencies starts every transitive depends_on dependency of name
// before starting name itself. The dependency graph is resolved from the
// provider hints (InstanceDependencies), validated to be acyclic, and walked so
// that each dependency reaches its declared condition before any instance that
// depends on it is started. Independent branches are started concurrently.
//
// If the graph contains a cycle (an invalid configuration), the dependencies are
// ignored with a warning and name is started on its own, mirroring how Docker
// Compose refuses a cyclic depends_on graph but still leaves the container
// startable.
func (s *Sablier) startWithDependencies(ctx context.Context, name string) error {
	graph := make(map[string][]InstanceDependency)
	if err := s.resolveDependencyGraph(ctx, name, graph); err != nil {
		return err
	}

	if cyclic, path := dependencyGraphCycle(name, graph); cyclic {
		s.l.WarnContext(ctx, "depends_on cycle detected, ignoring dependencies",
			slog.String("instance", name),
			slog.String("cycle", path),
		)
		return s.provider.InstanceStart(ctx, name)
	}

	// Start all of name's dependencies (recursively) and wait for their
	// conditions, then start name itself.
	if err := s.satisfyDependencies(ctx, name, graph, newDependencyMemo()); err != nil {
		return err
	}
	return s.provider.InstanceStart(ctx, name)
}

// resolveDependencyGraph populates graph with the direct dependencies of name
// and, recursively, of every dependency reachable from it. name is recorded
// before its dependencies are walked so a cyclic configuration terminates
// instead of recursing forever; the cycle is reported later by
// dependencyGraphCycle.
func (s *Sablier) resolveDependencyGraph(ctx context.Context, name string, graph map[string][]InstanceDependency) error {
	if _, ok := graph[name]; ok {
		return nil
	}

	deps, err := s.provider.InstanceDependencies(ctx, name)
	if err != nil {
		return fmt.Errorf("cannot resolve dependencies of %q: %w", name, err)
	}
	graph[name] = deps

	for _, dep := range deps {
		if err := s.resolveDependencyGraph(ctx, dep.Name, graph); err != nil {
			return err
		}
	}
	return nil
}

// dependencyGraphCycle reports whether the graph reachable from root contains a
// cycle and, if so, the offending path (e.g. "a -> b -> a"). It is a three-color
// depth-first search, the same approach Docker Compose uses to validate its
// dependency graph.
func dependencyGraphCycle(root string, graph map[string][]InstanceDependency) (bool, string) {
	const (
		visiting = 1
		visited  = 2
	)

	state := make(map[string]int, len(graph))

	var visit func(node string, path []string) (bool, string)
	visit = func(node string, path []string) (bool, string) {
		state[node] = visiting
		path = append(path, node)
		for _, dep := range graph[node] {
			switch state[dep.Name] {
			case visiting:
				return true, strings.Join(append(path, dep.Name), " -> ")
			case visited:
				continue
			default:
				if cyclic, p := visit(dep.Name, path); cyclic {
					return true, p
				}
			}
		}
		state[node] = visited
		return false, ""
	}

	return visit(root, nil)
}

// satisfyDependencies starts every direct dependency of node (recursively) and
// waits for each to reach its declared condition. Independent dependencies are
// processed concurrently; the memo guarantees each node is started at most once
// even when several dependents share it (a diamond graph).
func (s *Sablier) satisfyDependencies(ctx context.Context, node string, graph map[string][]InstanceDependency, memo *dependencyMemo) error {
	deps := graph[node]
	if len(deps) == 0 {
		return nil
	}

	g, gctx := errgroup.WithContext(ctx)
	for _, dep := range deps {
		g.Go(func() error {
			return s.satisfyDependency(gctx, dep, graph, memo)
		})
	}
	return g.Wait()
}

// satisfyDependency ensures a single dependency reaches its condition. A
// dependency that already satisfies its condition (e.g. an always-on service or
// a completed one-shot container) is left untouched, so Sablier never restarts a
// migration that has already done its work.
func (s *Sablier) satisfyDependency(ctx context.Context, dep InstanceDependency, graph map[string][]InstanceDependency, memo *dependencyMemo) error {
	if info, err := s.provider.InstanceInspect(ctx, dep.Name); err == nil &&
		dependencyConditionSatisfied(info.Status, dep.Condition) {
		return nil
	}

	if err := s.startDependencyNode(ctx, dep.Name, graph, memo); err != nil {
		return err
	}
	return s.waitForDependencyCondition(ctx, dep.Name, dep.Condition)
}

// startDependencyNode starts a single graph node after its own dependencies are
// satisfied. Work is memoized so the node is started exactly once per
// startWithDependencies call even if it is reached through several paths.
func (s *Sablier) startDependencyNode(ctx context.Context, node string, graph map[string][]InstanceDependency, memo *dependencyMemo) error {
	result, first := memo.load(node)
	if !first {
		select {
		case <-result.done:
			return result.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	err := s.satisfyDependencies(ctx, node, graph, memo)
	if err == nil {
		err = s.ensureDependencyStarted(ctx, node)
	}
	result.err = err
	close(result.done)
	return err
}

// ensureDependencyStarted issues at most one InstanceStart for name across all
// concurrent callers. When a Sablier-managed start is already in progress for
// name (it is a group member being started concurrently), it defers to that
// goroutine and returns without issuing a redundant start.
func (s *Sablier) ensureDependencyStarted(ctx context.Context, name string) error {
	s.pendingMu.Lock()
	if _, managed := s.pendingStarts[name]; managed {
		// A managed group-member start owns this instance's lifecycle; let it
		// perform the start. The caller still waits for the condition afterwards.
		s.pendingMu.Unlock()
		return nil
	}
	if ds, inflight := s.depStarts[name]; inflight {
		s.pendingMu.Unlock()
		select {
		case <-ds.done:
			return ds.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ds := &depStart{done: make(chan struct{})}
	s.depStarts[name] = ds
	s.pendingMu.Unlock()

	err := s.provider.InstanceStart(ctx, name)
	if err != nil {
		err = fmt.Errorf("cannot start dependency %q: %w", name, err)
	}

	s.pendingMu.Lock()
	ds.err = err
	if s.depStarts[name] == ds {
		delete(s.depStarts, name)
	}
	close(ds.done)
	s.pendingMu.Unlock()
	return err
}

// waitForDependencyCondition blocks until the named instance satisfies condition
// (as reported by provider.InstanceInspect), ctx is cancelled, or the dependency
// reports an error status.
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

// depStart tracks a single in-flight InstanceStart issued for a dependency so
// concurrent callers can wait on it instead of starting the same instance twice.
type depStart struct {
	done chan struct{}
	err  error
}

// dependencyMemo deduplicates node processing within one startWithDependencies
// call. Each node maps to a result whose done channel is closed once the node
// has been started (or has failed).
type dependencyMemo struct {
	mu      sync.Mutex
	results map[string]*dependencyResult
}

type dependencyResult struct {
	done chan struct{}
	err  error
}

func newDependencyMemo() *dependencyMemo {
	return &dependencyMemo{results: make(map[string]*dependencyResult)}
}

// load returns the result for node and whether the caller is the first to
// request it. The first caller is responsible for starting the node and closing
// the result's done channel; subsequent callers wait on it.
func (m *dependencyMemo) load(node string) (*dependencyResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.results[node]; ok {
		return r, false
	}
	r := &dependencyResult{done: make(chan struct{})}
	m.results[node] = r
	return r, true
}
