package docker

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
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
// dependencyEdge is a directed edge "from depends on to" in the dependency
// graph. Both endpoints are container names.
type dependencyEdge struct {
	from string
	to   string
}

// dependencyGraph is the global, concurrency-safe dependency graph for the
// provider. It is a Directed Acyclic Graph (DAG): each time an instance is
// started, its dependency tree is committed here and rejected if it would
// introduce a cycle.
//
// adjacency stores the live edges with a reference count so that edges shared
// between multiple instance trees are only removed once the last contributing
// tree is replaced. byRoot remembers the exact set of edges each instance tree
// previously contributed so that re-committing an instance replaces (rather
// than duplicates) its edges.
type dependencyGraph struct {
	mu        sync.Mutex
	adjacency map[string]map[string]int
	byRoot    map[string][]dependencyEdge
}

func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{
		adjacency: make(map[string]map[string]int),
		byRoot:    make(map[string][]dependencyEdge),
	}
}

func (g *dependencyGraph) addEdgeLocked(e dependencyEdge) {
	to := g.adjacency[e.from]
	if to == nil {
		to = make(map[string]int)
		g.adjacency[e.from] = to
	}
	to[e.to]++
}

func (g *dependencyGraph) removeEdgeLocked(e dependencyEdge) {
	to := g.adjacency[e.from]
	if to == nil {
		return
	}
	to[e.to]--
	if to[e.to] <= 0 {
		delete(to, e.to)
	}
	if len(to) == 0 {
		delete(g.adjacency, e.from)
	}
}

// reachableLocked reports whether to is reachable from from following the
// current edges.
func (g *dependencyGraph) reachableLocked(from, to string) bool {
	if from == to {
		return true
	}
	visited := make(map[string]struct{})
	stack := []string{from}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := visited[n]; ok {
			continue
		}
		visited[n] = struct{}{}
		for next := range g.adjacency[n] {
			if next == to {
				return true
			}
			stack = append(stack, next)
		}
	}
	return false
}

// commit replaces the edges previously contributed by root with the given
// edges, provided the graph remains a DAG. If adding any edge would introduce a
// self-loop or a cycle, the graph is rolled back to its prior valid state and a
// descriptive error is returned.
func (g *dependencyGraph) commit(root string, edges []dependencyEdge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	previous := g.byRoot[root]
	for _, e := range previous {
		g.removeEdgeLocked(e)
	}

	added := make([]dependencyEdge, 0, len(edges))
	for _, e := range edges {
		if e.from == e.to {
			g.rollbackLocked(added, previous, root)
			return fmt.Errorf("container %q cannot depend on itself", e.from)
		}
		// Adding from->to creates a cycle if (and only if) to can already reach
		// from in the graph built so far.
		if g.reachableLocked(e.to, e.from) {
			g.rollbackLocked(added, previous, root)
			return fmt.Errorf("dependency %q -> %q would create a cycle", e.from, e.to)
		}
		g.addEdgeLocked(e)
		added = append(added, e)
	}

	g.byRoot[root] = edges
	return nil
}

func (g *dependencyGraph) rollbackLocked(added, previous []dependencyEdge, root string) {
	for _, e := range added {
		g.removeEdgeLocked(e)
	}
	for _, e := range previous {
		g.addEdgeLocked(e)
	}
	g.byRoot[root] = previous
}

// treeDep is a single resolved depends_on dependency of a node: the dependency
// container name and the condition that must be satisfied before the dependent
// container is started.
type treeDep struct {
	name      string
	condition string
}

// dependencyTree is the resolved depends_on tree rooted at a single instance.
// nodes maps each container name to its direct dependencies. Every node that
// appears anywhere in the tree has an entry (leaves map to an empty slice).
type dependencyTree struct {
	root  string
	nodes map[string][]treeDep
}

// edges returns the directed edges of the tree for committing to the global
// dependency graph.
func (t *dependencyTree) edges() []dependencyEdge {
	var edges []dependencyEdge
	for from, deps := range t.nodes {
		for _, dep := range deps {
			edges = append(edges, dependencyEdge{from: from, to: dep.name})
		}
	}
	return edges
}

// buildDependencyTree walks the Docker Compose depends_on dependencies starting
// from root and builds the resolved dependency tree. Dependencies that cannot
// be resolved to a container (for example external services not managed by this
// Compose project) are skipped with a warning.
//
// A node is recorded before its dependencies are walked, so a cyclic compose
// definition does not cause infinite recursion: the cycle is recorded in full
// and later detected when the tree is committed to the global dependency graph.
func (p *Provider) buildDependencyTree(ctx context.Context, root string) (*dependencyTree, error) {
	tree := &dependencyTree{nodes: make(map[string][]treeDep)}

	var walk func(name string) (string, error)
	walk = func(name string) (string, error) {
		spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
		if err != nil {
			return "", fmt.Errorf("cannot inspect container: %w", err)
		}

		canonical := strings.TrimPrefix(spec.Container.Name, "/")
		if _, ok := tree.nodes[canonical]; ok {
			return canonical, nil
		}

		labels := spec.Container.Config.Labels
		project := labels[composeProjectLabel]
		deps := parseComposeDependsOn(labels[composeDependsOnLabel])

		nodeDeps := make([]treeDep, 0, len(deps))
		for _, dep := range deps {
			depName, err := p.findComposeContainer(ctx, project, dep.Service)
			if err != nil {
				return "", err
			}
			if depName == "" {
				p.l.WarnContext(ctx, "skipping depends_on dependency, no container found",
					slog.String("service", dep.Service),
					slog.String("project", project),
				)
				continue
			}
			nodeDeps = append(nodeDeps, treeDep{name: depName, condition: dep.Condition})
		}

		// Record the node before recursing to break cycles while building.
		tree.nodes[canonical] = nodeDeps

		for _, dep := range nodeDeps {
			if _, err := walk(dep.name); err != nil {
				return "", err
			}
		}

		return canonical, nil
	}

	rootName, err := walk(root)
	if err != nil {
		return nil, err
	}
	tree.root = rootName
	return tree, nil
}

// startTree starts every container in the validated dependency tree in
// dependency order: a container is started only once all of its depends_on
// dependencies have been started and have satisfied their declared condition.
func (p *Provider) startTree(ctx context.Context, tree *dependencyTree) error {
	return p.resolveNode(ctx, tree, tree.root, make(map[string]struct{}))
}

// resolveNode starts node after recursively starting and waiting for its
// dependencies. The tree is guaranteed acyclic (validated on commit), so the
// recursion always terminates; started guards against starting a shared
// dependency more than once.
func (p *Provider) resolveNode(ctx context.Context, tree *dependencyTree, node string, started map[string]struct{}) error {
	if _, ok := started[node]; ok {
		return nil
	}

	for _, dep := range tree.nodes[node] {
		if err := p.resolveNode(ctx, tree, dep.name, started); err != nil {
			return err
		}
		p.l.DebugContext(ctx, "waiting for depends_on dependency",
			slog.String("dependency", dep.name),
			slog.String("condition", dep.condition),
		)
		if err := p.waitForDependencyCondition(ctx, dep.name, dep.condition); err != nil {
			return fmt.Errorf("dependency %s did not satisfy condition %q: %w", dep.name, dep.condition, err)
		}
	}

	if err := p.startSingle(ctx, node); err != nil {
		return err
	}
	started[node] = struct{}{}
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
