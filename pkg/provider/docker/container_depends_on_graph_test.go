package docker

// Unit tests for the global dependency graph (DAG) used to validate and store
// each instance's depends_on tree. These run without a real Docker daemon.

import "testing"

func edges(pairs ...[2]string) []dependencyEdge {
	es := make([]dependencyEdge, 0, len(pairs))
	for _, p := range pairs {
		es = append(es, dependencyEdge{from: p[0], to: p[1]})
	}
	return es
}

func TestDependencyGraph_Commit_AcyclicChain(t *testing.T) {
	g := newDependencyGraph()

	// app -> migration -> db (a valid chain).
	err := g.commit("app", edges([2]string{"app", "migration"}, [2]string{"migration", "db"}))
	if err != nil {
		t.Fatalf("expected acyclic chain to commit, got error: %v", err)
	}
}

func TestDependencyGraph_Commit_RejectsSelfLoop(t *testing.T) {
	g := newDependencyGraph()

	err := g.commit("app", edges([2]string{"app", "app"}))
	if err == nil {
		t.Fatal("expected self-loop to be rejected")
	}

	if len(g.adjacency) != 0 {
		t.Fatalf("expected graph to be left empty after rollback, got %v", g.adjacency)
	}
}

func TestDependencyGraph_Commit_RejectsCycle(t *testing.T) {
	g := newDependencyGraph()

	// a -> b -> c -> a forms a cycle.
	err := g.commit("a", edges(
		[2]string{"a", "b"},
		[2]string{"b", "c"},
		[2]string{"c", "a"},
	))
	if err == nil {
		t.Fatal("expected cycle to be rejected")
	}

	// The whole tree must be ignored: the graph is rolled back to empty.
	if len(g.adjacency) != 0 {
		t.Fatalf("expected graph to be empty after rejecting cyclic tree, got %v", g.adjacency)
	}
}

func TestDependencyGraph_Commit_RejectsCycleAcrossRoots(t *testing.T) {
	g := newDependencyGraph()

	if err := g.commit("a", edges([2]string{"a", "b"})); err != nil {
		t.Fatalf("unexpected error committing a->b: %v", err)
	}

	// b -> a would close a cycle with the already-committed a -> b.
	if err := g.commit("b", edges([2]string{"b", "a"})); err == nil {
		t.Fatal("expected cross-root cycle to be rejected")
	}

	// The first tree must remain intact.
	if !g.reachableLocked("a", "b") {
		t.Fatal("expected a -> b to still be present after rejecting b -> a")
	}
}

func TestDependencyGraph_Commit_SharedDependencyRefcount(t *testing.T) {
	g := newDependencyGraph()

	// Two independent roots both depend on db (diamond-style sharing).
	if err := g.commit("app", edges([2]string{"app", "db"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := g.commit("worker", edges([2]string{"worker", "db"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-committing app with no edges must not remove worker -> db.
	if err := g.commit("app", nil); err != nil {
		t.Fatalf("unexpected error recommitting app: %v", err)
	}
	if !g.reachableLocked("worker", "db") {
		t.Fatal("expected worker -> db to remain after app re-commit")
	}
}

func TestDependencyGraph_Commit_ReplacesPreviousTree(t *testing.T) {
	g := newDependencyGraph()

	if err := g.commit("app", edges([2]string{"app", "db"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-commit app with a different dependency: db edge must be gone.
	if err := g.commit("app", edges([2]string{"app", "cache"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.reachableLocked("app", "db") {
		t.Fatal("expected app -> db to be replaced by app -> cache")
	}
	if !g.reachableLocked("app", "cache") {
		t.Fatal("expected app -> cache after re-commit")
	}
}
