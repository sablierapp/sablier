package docker

import "fmt"

// topoSort returns the elements of nodes sorted in topological order so that
// each node appears before the nodes that depend on it. deps maps a node name
// to the set of nodes it directly depends on. Only edges between nodes that
// exist in the nodes slice are respected; external dependencies are ignored.
//
// Returns an error if a cycle is detected.
func topoSort(nodes []string, deps map[string][]string) ([]string, error) {
	// Build a set of known nodes for fast lookup.
	inSet := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		inSet[n] = true
	}

	// Build the in-degree map and adjacency list (only for known nodes).
	inDegree := make(map[string]int, len(nodes))
	adj := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		inDegree[n] = 0
	}
	for n, nodeDeps := range deps {
		if !inSet[n] {
			continue
		}
		for _, dep := range nodeDeps {
			if !inSet[dep] {
				continue // dependency not in this group, ignore
			}
			// n depends on dep → dep must come before n → edge dep→n
			adj[dep] = append(adj[dep], n)
			inDegree[n]++
		}
	}

	// Kahn's algorithm.
	queue := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}

	sorted := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		// Pick the first node with in-degree 0.
		n := queue[0]
		queue = queue[1:]
		sorted = append(sorted, n)
		for _, m := range adj[n] {
			inDegree[m]--
			if inDegree[m] == 0 {
				queue = append(queue, m)
			}
		}
	}

	if len(sorted) != len(nodes) {
		return nil, fmt.Errorf("cycle detected in sablier.depends-on relationships")
	}
	return sorted, nil
}
