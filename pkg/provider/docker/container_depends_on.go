package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moby/moby/client"
)

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
			return "", fmt.Errorf("cannot inspect container: %w", err)
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

// startTree starts every container in tree in dependency order: a container is
// started only after each of its dependencies has started and satisfied its
// condition. tree must be acyclic.
func (p *Provider) startTree(ctx context.Context, tree *dependencyTree) error {
	return p.resolveNode(ctx, tree, tree.root, make(map[string]struct{}))
}

// resolveNode starts node after starting and waiting for its dependencies.
// started guards against starting a shared dependency more than once.
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
