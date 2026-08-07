package sablier

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"

	"github.com/sablierapp/sablier/pkg/provider"
)

// fakeDepProvider is a controllable in-memory provider used to exercise the core
// depends_on orchestration (graph resolution, ordering, concurrency, cycles)
// without a real container runtime. Only the methods used by
// startWithDependencies do real work; the rest are stubs.
type fakeDepProvider struct {
	mu sync.Mutex

	// deps maps an instance to its direct dependencies.
	deps map[string][]InstanceDependency
	// status is the current status reported by InstanceInspect. Missing entries
	// default to statusDefault.
	status        map[string]InstanceStatus
	statusDefault InstanceStatus
	// onStart is the status an instance transitions to once InstanceStart
	// succeeds. Missing entries default to InstanceStatusReady.
	onStart map[string]InstanceStatus

	startErr   map[string]error
	inspectErr map[string]error
	depsErr    map[string]error
	startDelay time.Duration

	// recorded observations
	startOrder []string
	startCount map[string]int
}

func newFakeDepProvider() *fakeDepProvider {
	return &fakeDepProvider{
		deps:          map[string][]InstanceDependency{},
		status:        map[string]InstanceStatus{},
		statusDefault: InstanceStatusStopped,
		onStart:       map[string]InstanceStatus{},
		startErr:      map[string]error{},
		inspectErr:    map[string]error{},
		depsErr:       map[string]error{},
		startCount:    map[string]int{},
	}
}

func (f *fakeDepProvider) InstanceStart(_ context.Context, name string) error {
	f.mu.Lock()
	f.startOrder = append(f.startOrder, name)
	f.startCount[name]++
	delay := f.startDelay
	err := f.startErr[name]
	f.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if err != nil {
		return err
	}

	f.mu.Lock()
	next, ok := f.onStart[name]
	if !ok {
		next = InstanceStatusReady
	}
	f.status[name] = next
	f.mu.Unlock()
	return nil
}

func (f *fakeDepProvider) InstanceInspect(_ context.Context, name string) (InstanceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.inspectErr[name]; err != nil {
		return InstanceInfo{}, err
	}
	st, ok := f.status[name]
	if !ok {
		st = f.statusDefault
	}
	return InstanceInfo{Name: name, Status: st}, nil
}

func (f *fakeDepProvider) InstanceDependencies(_ context.Context, name string) ([]InstanceDependency, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.depsErr[name]; err != nil {
		return nil, err
	}
	return f.deps[name], nil
}

func (f *fakeDepProvider) starts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.startOrder))
	copy(out, f.startOrder)
	return out
}

func (f *fakeDepProvider) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCount[name]
}

// Unused interface methods.
func (f *fakeDepProvider) InstanceStop(context.Context, string) error { return nil }
func (f *fakeDepProvider) InstanceGroups(context.Context) (map[string][]string, error) {
	return nil, nil
}
func (f *fakeDepProvider) InstanceList(context.Context, provider.InstanceListOptions) ([]InstanceConfiguration, error) {
	return nil, nil
}
func (f *fakeDepProvider) InstanceEvents(context.Context, provider.InstanceEventsOptions) InstanceEventStream {
	return InstanceEventStream{}
}

func newDepSablier(t *testing.T, fp *fakeDepProvider) *Sablier {
	t.Helper()
	return New(slogt.New(t), nil, fp)
}

func condStarted(name string) InstanceDependency {
	return InstanceDependency{Name: name, Condition: "service_started"}
}

func TestStartWithDependencies_NoDeps(t *testing.T) {
	fp := newFakeDepProvider()
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fp.starts(); len(got) != 1 || got[0] != "app" {
		t.Fatalf("expected only app to be started, got %v", got)
	}
}

func TestStartWithDependencies_ChainStartsDepsFirst(t *testing.T) {
	// app -> migration -> db. Expect start order db, migration, app.
	fp := newFakeDepProvider()
	fp.deps["app"] = []InstanceDependency{condStarted("migration")}
	fp.deps["migration"] = []InstanceDependency{condStarted("db")}
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := fp.starts()
	want := []string{"db", "migration", "app"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected start order %v, got %v", want, got)
		}
	}
}

func TestStartWithDependencies_DiamondStartsSharedDepOnce(t *testing.T) {
	// app -> {web, worker} -> db. db must be started exactly once.
	fp := newFakeDepProvider()
	fp.deps["app"] = []InstanceDependency{condStarted("web"), condStarted("worker")}
	fp.deps["web"] = []InstanceDependency{condStarted("db")}
	fp.deps["worker"] = []InstanceDependency{condStarted("db")}
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"app", "web", "worker", "db"} {
		if c := fp.count(name); c != 1 {
			t.Fatalf("expected %s to start exactly once, started %d times (order %v)", name, c, fp.starts())
		}
	}
	got := fp.starts()
	if got[0] != "db" {
		t.Fatalf("expected db to start first, got %v", got)
	}
	if got[len(got)-1] != "app" {
		t.Fatalf("expected app to start last, got %v", got)
	}
}

func TestStartWithDependencies_CycleStartsAlone(t *testing.T) {
	// a -> b -> a. The cycle is ignored and only a is started.
	fp := newFakeDepProvider()
	fp.deps["a"] = []InstanceDependency{condStarted("b")}
	fp.deps["b"] = []InstanceDependency{condStarted("a")}
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fp.starts(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("expected only a to be started on a cyclic graph, got %v", got)
	}
}

func TestStartWithDependencies_CompletedDepNotRestarted(t *testing.T) {
	// migration is an already-completed one-shot. It must not be restarted.
	fp := newFakeDepProvider()
	fp.deps["app"] = []InstanceDependency{{Name: "migration", Condition: "service_completed_successfully"}}
	fp.status["migration"] = InstanceStatusCompleted
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c := fp.count("migration"); c != 0 {
		t.Fatalf("expected completed migration to not be restarted, started %d times", c)
	}
	if c := fp.count("app"); c != 1 {
		t.Fatalf("expected app to be started once, started %d times", c)
	}
}

func TestStartWithDependencies_WaitsForCompletion(t *testing.T) {
	// migration starts Stopped and transitions to Completed once started; the
	// service_completed_successfully condition must resolve.
	fp := newFakeDepProvider()
	fp.deps["app"] = []InstanceDependency{{Name: "migration", Condition: "service_completed_successfully"}}
	fp.onStart["migration"] = InstanceStatusCompleted
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "app"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fp.starts()
	if len(got) != 2 || got[0] != "migration" || got[1] != "app" {
		t.Fatalf("expected [migration app], got %v", got)
	}
}

func TestStartWithDependencies_PropagatesStartError(t *testing.T) {
	fp := newFakeDepProvider()
	fp.deps["app"] = []InstanceDependency{condStarted("db")}
	fp.startErr["db"] = errors.New("boom")
	s := newDepSablier(t, fp)

	err := s.startWithDependencies(context.Background(), "app")
	if err == nil {
		t.Fatal("expected an error when a dependency fails to start")
	}
	if c := fp.count("app"); c != 0 {
		t.Fatalf("app must not start when a dependency fails, started %d times", c)
	}
}

func TestEnsureDependencyStarted_SingleFlight(t *testing.T) {
	fp := newFakeDepProvider()
	fp.startDelay = 50 * time.Millisecond
	s := newDepSablier(t, fp)

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_ = s.ensureDependencyStarted(context.Background(), "db")
		}()
	}
	wg.Wait()

	if c := fp.count("db"); c != 1 {
		t.Fatalf("expected a single InstanceStart across concurrent callers, got %d", c)
	}
}

func TestEnsureDependencyStarted_DefersToManagedStart(t *testing.T) {
	fp := newFakeDepProvider()
	s := newDepSablier(t, fp)

	// Simulate a group member start already in progress for db.
	s.pendingMu.Lock()
	s.pendingStarts["db"] = &pendingStart{done: make(chan struct{})}
	s.pendingMu.Unlock()

	if err := s.ensureDependencyStarted(context.Background(), "db"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c := fp.count("db"); c != 0 {
		t.Fatalf("expected no start when a managed start owns the instance, got %d", c)
	}
}

func TestWaitForDependencyCondition_ErrorStatus(t *testing.T) {
	fp := newFakeDepProvider()
	fp.status["db"] = InstanceStatusError
	s := newDepSablier(t, fp)

	err := s.waitForDependencyCondition(context.Background(), "db", "service_healthy")
	if err == nil {
		t.Fatal("expected an error when the dependency is in error state")
	}
}

func TestWaitForDependencyCondition_ContextCancelled(t *testing.T) {
	fp := newFakeDepProvider()
	fp.status["db"] = InstanceStatusStarting // never satisfies service_healthy
	s := newDepSablier(t, fp)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := s.waitForDependencyCondition(ctx, "db", "service_healthy")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
}

func TestResolveDependencyGraph_BuildsReachableNodes(t *testing.T) {
	fp := newFakeDepProvider()
	fp.deps["app"] = []InstanceDependency{condStarted("web"), condStarted("worker")}
	fp.deps["web"] = []InstanceDependency{condStarted("db")}
	fp.deps["worker"] = []InstanceDependency{condStarted("db")}
	s := newDepSablier(t, fp)

	graph := map[string][]InstanceDependency{}
	if err := s.resolveDependencyGraph(context.Background(), "app", graph); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, node := range []string{"app", "web", "worker", "db"} {
		if _, ok := graph[node]; !ok {
			t.Fatalf("expected node %q in resolved graph %v", node, graph)
		}
	}
}

func TestStartWithDependencies_PropagatesResolveError(t *testing.T) {
	fp := newFakeDepProvider()
	fp.depsErr["app"] = errors.New("cannot inspect")
	s := newDepSablier(t, fp)

	if err := s.startWithDependencies(context.Background(), "app"); err == nil {
		t.Fatal("expected an error when dependency resolution fails")
	}
	if c := fp.count("app"); c != 0 {
		t.Fatalf("app must not start when resolution fails, started %d times", c)
	}
}

func TestResolveDependencyGraph_TerminatesOnCycle(t *testing.T) {
	fp := newFakeDepProvider()
	fp.deps["a"] = []InstanceDependency{condStarted("b")}
	fp.deps["b"] = []InstanceDependency{condStarted("a")}
	s := newDepSablier(t, fp)

	graph := map[string][]InstanceDependency{}
	if err := s.resolveDependencyGraph(context.Background(), "a", graph); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph) != 2 {
		t.Fatalf("expected 2 nodes for a cyclic graph, got %v", graph)
	}
}

// graphOf builds a dependency graph (all edges use service_started) for cycle tests.
func graphOf(nodes map[string][]string) map[string][]InstanceDependency {
	g := make(map[string][]InstanceDependency, len(nodes))
	for from, tos := range nodes {
		deps := make([]InstanceDependency, 0, len(tos))
		for _, to := range tos {
			deps = append(deps, condStarted(to))
		}
		g[from] = deps
	}
	return g
}

func TestDependencyGraphCycle(t *testing.T) {
	tests := []struct {
		name      string
		root      string
		graph     map[string][]string
		wantCycle bool
	}{
		{name: "acyclic chain", root: "app", graph: map[string][]string{"app": {"migration"}, "migration": {"db"}, "db": nil}},
		{name: "diamond", root: "app", graph: map[string][]string{"app": {"web", "worker"}, "web": {"db"}, "worker": {"db"}, "db": nil}},
		{name: "single node", root: "app", graph: map[string][]string{"app": nil}},
		{name: "self loop", root: "app", graph: map[string][]string{"app": {"app"}}, wantCycle: true},
		{name: "direct cycle", root: "app", graph: map[string][]string{"app": {"db"}, "db": {"app"}}, wantCycle: true},
		{name: "indirect cycle", root: "a", graph: map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}, wantCycle: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cyclic, path := dependencyGraphCycle(tt.root, graphOf(tt.graph))
			if cyclic != tt.wantCycle {
				t.Fatalf("dependencyGraphCycle = %v (path %q), want %v", cyclic, path, tt.wantCycle)
			}
			if cyclic && path == "" {
				t.Fatal("expected a non-empty cycle path")
			}
		})
	}
}
