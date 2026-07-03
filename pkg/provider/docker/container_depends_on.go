package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moby/moby/client"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// InstanceDependencies returns the direct depends_on dependencies of name by
// reading the container's com.docker.compose.depends_on label. It resolves each
// declared service to a concrete container name within the same Compose project.
//
// Only direct dependencies are returned; Sablier core is responsible for walking
// the graph transitively, detecting cycles, and ordering the starts. Providers
// that cannot express dependencies return nil, nil.
//
// Dependencies whose service has no matching container, or containers without a
// Compose project label, are skipped with a warning so a start is never blocked
// by an unresolvable or unrelated dependency.
func (p *Provider) InstanceDependencies(ctx context.Context, name string) ([]sablier.InstanceDependency, error) {
	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot inspect dependency container %q: %w", name, err)
	}

	labels := spec.Container.Config.Labels
	raw := parseComposeDependsOn(labels[composeDependsOnLabel])
	if len(raw) == 0 {
		return nil, nil
	}

	// depends_on can only be resolved within a Compose project. Without the
	// project label a service name could match a container from any other
	// project, so resolution is skipped to avoid starting unrelated containers.
	project := labels[composeProjectLabel]
	if project == "" {
		p.l.WarnContext(ctx, "skipping depends_on resolution, container has no compose project label",
			slog.String("container", strings.TrimPrefix(spec.Container.Name, "/")),
		)
		return nil, nil
	}

	var deps []sablier.InstanceDependency
	for _, dep := range raw {
		depName, err := p.findComposeContainer(ctx, project, dep.Service)
		if err != nil {
			return nil, err
		}
		if depName == "" {
			p.l.WarnContext(ctx, "skipping depends_on dependency, no container found",
				slog.String("service", dep.Service),
				slog.String("project", project),
			)
			continue
		}
		deps = append(deps, sablier.InstanceDependency{Name: depName, Condition: dep.Condition})
	}

	return deps, nil
}
