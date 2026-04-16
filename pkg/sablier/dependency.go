package sablier

import "fmt"

// ResolveDependencyOrder returns instances in dependency start order (deepest dependencies first)
// for the given target instance. The target itself is included as the last element.
// Returns an error if a cycle is detected.
func ResolveDependencyOrder(target string, deps map[string][]string) ([]string, error) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var order []string

	var visit func(node string) error
	visit = func(node string) error {
		if inStack[node] {
			return fmt.Errorf("dependency cycle detected involving %q", node)
		}
		if visited[node] {
			return nil
		}

		inStack[node] = true
		for _, dep := range deps[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		inStack[node] = false
		visited[node] = true
		order = append(order, node)
		return nil
	}

	if err := visit(target); err != nil {
		return nil, err
	}

	return order, nil
}

// ResolveDependents returns all instances that depend on the given target,
// directly or transitively (i.e., services that would break if target stops).
func ResolveDependents(target string, deps map[string][]string) []string {
	// Build reverse dependency map: for each dependency, track who depends on it.
	reverse := make(map[string][]string)
	for instance, dependencies := range deps {
		for _, dep := range dependencies {
			reverse[dep] = append(reverse[dep], instance)
		}
	}

	// BFS from target through the reverse graph.
	visited := make(map[string]bool)
	queue := []string{target}
	visited[target] = true

	var dependents []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, dependent := range reverse[current] {
			if !visited[dependent] {
				visited[dependent] = true
				dependents = append(dependents, dependent)
				queue = append(queue, dependent)
			}
		}
	}

	return dependents
}
