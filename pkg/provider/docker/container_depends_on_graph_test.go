package docker

// Unit tests for cycle detection on the resolved depends_on tree. These run
// without a real Docker daemon.

import "testing"

func tree(root string, nodes map[string][]string) *dependencyTree {
	t := &dependencyTree{root: root, nodes: make(map[string][]treeDep, len(nodes))}
	for from, deps := range nodes {
		td := make([]treeDep, 0, len(deps))
		for _, d := range deps {
			td = append(td, treeDep{name: d, condition: conditionServiceStarted})
		}
		t.nodes[from] = td
	}
	return t
}

func TestDependencyTree_HasCycle_AcyclicChain(t *testing.T) {
	// app -> migration -> db
	tr := tree("app", map[string][]string{
		"app":       {"migration"},
		"migration": {"db"},
		"db":        nil,
	})

	if cyclic, path := tr.hasCycle(); cyclic {
		t.Fatalf("expected acyclic chain, got cycle: %s", path)
	}
}

func TestDependencyTree_HasCycle_Diamond(t *testing.T) {
	// app depends on both web and worker, which both depend on db (shared).
	tr := tree("app", map[string][]string{
		"app":    {"web", "worker"},
		"web":    {"db"},
		"worker": {"db"},
		"db":     nil,
	})

	if cyclic, path := tr.hasCycle(); cyclic {
		t.Fatalf("expected acyclic diamond, got cycle: %s", path)
	}
}

func TestDependencyTree_HasCycle_SelfLoop(t *testing.T) {
	tr := tree("app", map[string][]string{
		"app": {"app"},
	})

	cyclic, path := tr.hasCycle()
	if !cyclic {
		t.Fatal("expected self-loop to be detected as a cycle")
	}
	if path == "" {
		t.Fatal("expected a non-empty cycle path")
	}
}

func TestDependencyTree_HasCycle_DirectCycle(t *testing.T) {
	// app -> db -> app
	tr := tree("app", map[string][]string{
		"app": {"db"},
		"db":  {"app"},
	})

	if cyclic, _ := tr.hasCycle(); !cyclic {
		t.Fatal("expected direct cycle to be detected")
	}
}

func TestDependencyTree_HasCycle_IndirectCycle(t *testing.T) {
	// a -> b -> c -> a
	tr := tree("a", map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	})

	if cyclic, _ := tr.hasCycle(); !cyclic {
		t.Fatal("expected indirect cycle to be detected")
	}
}

func TestDependencyTree_HasCycle_SingleNode(t *testing.T) {
	tr := &dependencyTree{root: "app", nodes: map[string][]treeDep{"app": nil}}

	if cyclic, path := tr.hasCycle(); cyclic {
		t.Fatalf("expected no cycle for single node, got: %s", path)
	}
}
