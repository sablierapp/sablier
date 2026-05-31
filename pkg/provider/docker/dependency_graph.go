package docker

import "strings"

// dependency is a resolved depends_on edge: the dependency container name and
// the condition that must hold before the dependent container is started.
type dependency struct {
	name      string
	condition string
}

// dependencyTree is a resolved depends_on graph rooted at a single container.
// nodes maps every container in the tree to its direct dependencies; leaves map
// to an empty slice.
//
// A Compose depends_on graph is expected to be acyclic, but a misconfiguration
// can introduce a cycle, so callers validate the tree with hasCycle before
// walking it.
type dependencyTree struct {
	root  string
	nodes map[string][]dependency
}

// hasCycle reports whether the tree contains a cycle and, if so, the offending
// path (e.g. "a -> b -> a"). It is a three-color depth-first search, the same
// approach Docker Compose uses to validate its dependency graph.
func (t *dependencyTree) hasCycle() (bool, string) {
	const (
		visiting = 1
		visited  = 2
	)

	state := make(map[string]int, len(t.nodes))

	var visit func(node string, path []string) (bool, string)
	visit = func(node string, path []string) (bool, string) {
		state[node] = visiting
		path = append(path, node)
		for _, dep := range t.nodes[node] {
			switch state[dep.name] {
			case visiting:
				return true, strings.Join(append(path, dep.name), " -> ")
			case visited:
				continue
			default:
				if cyclic, p := visit(dep.name, path); cyclic {
					return true, p
				}
			}
		}
		state[node] = visited
		return false, ""
	}

	for node := range t.nodes {
		if state[node] == 0 {
			if cyclic, p := visit(node, nil); cyclic {
				return true, p
			}
		}
	}
	return false, ""
}
