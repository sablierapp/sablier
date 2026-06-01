package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moby/moby/client"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// InstanceDependencies returns the transitive depends_on dependencies of name
// in topological order by inspecting the container's Docker Compose labels.
// Each dependency is listed before the nodes that depend on it, so Sablier core
// can iterate the slice and start each entry in order.
//
// If the dependency graph contains a cycle (invalid Compose configuration), an
// empty slice is returned with a warning log so the caller can still attempt to
// start the instance on its own.
func (p *Provider) InstanceDependencies(ctx context.Context, name string) ([]sablier.InstanceDependency, error) {
	tree, err := p.buildDependencyTree(ctx, name)
	if err != nil {
		return nil, err
	}
	if cyclic, path := tree.hasCycle(); cyclic {
		p.l.WarnContext(ctx, "depends_on cycle detected, ignoring dependencies",
			slog.String("instance", name),
			slog.String("cycle", path),
		)
		return nil, nil
	}
	return tree.topologicalDependencies(), nil
}

// buildDependencyTree resolves the depends_on graph reachable from root by
// inspecting containers and following their com.docker.compose.depends_on
// labels. Dependencies with no matching container are skipped with a warning.
//
// Each node is recorded before its dependencies are walked, so a cyclic Compose
// definition terminates instead of recursing forever; the cycle is then
// reported by dependencyTree.hasCycle.
func (p *Provider) buildDependencyTree(ctx context.Context, root string) (*dependencyTree, error) {
	tree := &dependencyTree{nodes: make(map[string][]dependency)}

	var walk func(name string) (string, error)
	walk = func(name string) (string, error) {
		spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
		if err != nil {
			return "", fmt.Errorf("cannot inspect dependency container %q: %w", name, err)
		}

		canonical := strings.TrimPrefix(spec.Container.Name, "/")
		if _, ok := tree.nodes[canonical]; ok {
			return canonical, nil
		}

		labels := spec.Container.Config.Labels
		project := labels[composeProjectLabel]

		// depends_on can only be resolved within a Compose project. Without the
		// project label, a service name could match a container from any other
		// project, so resolution is skipped to avoid starting unrelated
		// containers.
		raw := parseComposeDependsOn(labels[composeDependsOnLabel])
		if len(raw) > 0 && project == "" {
			p.l.WarnContext(ctx, "skipping depends_on resolution, container has no compose project label",
				slog.String("container", canonical),
			)
			tree.nodes[canonical] = nil
			return canonical, nil
		}

		var deps []dependency
		for _, dep := range raw {
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
			deps = append(deps, dependency{name: depName, condition: dep.Condition})
		}

		tree.nodes[canonical] = deps
		for _, dep := range deps {
			if _, err := walk(dep.name); err != nil {
				return "", err
			}
		}
		return canonical, nil
	}

	root, err := walk(root)
	if err != nil {
		return nil, err
	}
	tree.root = root
	return tree, nil
}

