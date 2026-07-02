package docker

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Docker Compose labels set on every container it manages.
const (
	composeProjectLabel   = "com.docker.compose.project"
	composeServiceLabel   = "com.docker.compose.service"
	composeDependsOnLabel = "com.docker.compose.depends_on"
)

// composeDependency is one entry of the com.docker.compose.depends_on label,
// referencing a service by name (not yet resolved to a container).
type composeDependency struct {
	Service   string
	Condition string
}

// parseComposeDependsOn parses the com.docker.compose.depends_on label, a
// comma-separated list of "service:condition:restart" entries. Malformed
// entries are skipped.
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

// findComposeContainer returns the container name for a Compose project and
// service, or "" when none exists. When several containers match (scaled
// services or leftovers from a previous run), a running one is preferred,
// otherwise the lexicographically smallest name, so the choice is deterministic.
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
