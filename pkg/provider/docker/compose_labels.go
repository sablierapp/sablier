package docker

import "strings"

// Docker Compose label constants.
const (
	ComposeProjectLabel      = "com.docker.compose.project"
	ComposeServiceLabel      = "com.docker.compose.service"
	ComposeDependenciesLabel = "com.docker.compose.depends_on"
)

// DependsOnEntry represents a single entry from the com.docker.compose.depends_on label.
type DependsOnEntry struct {
	Service   string
	Condition string
	Restart   string
}

// ComposeMetadata holds the Docker Compose labels extracted from a container.
type ComposeMetadata struct {
	Project   string
	Service   string
	DependsOn []DependsOnEntry
}

// parseComposeLabels extracts Docker Compose metadata from container labels.
func parseComposeLabels(labels map[string]string) ComposeMetadata {
	meta := ComposeMetadata{
		Project: labels[ComposeProjectLabel],
		Service: labels[ComposeServiceLabel],
	}

	if deps, ok := labels[ComposeDependenciesLabel]; ok && deps != "" {
		meta.DependsOn = parseDependsOn(deps)
	}

	return meta
}

// parseDependsOn parses the com.docker.compose.depends_on label value.
// The format is "svc:condition:restart,svc2:condition:restart,...".
func parseDependsOn(value string) []DependsOnEntry {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	entries := make([]DependsOnEntry, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.SplitN(part, ":", 3)
		entry := DependsOnEntry{
			Service: fields[0],
		}
		if len(fields) > 1 {
			entry.Condition = fields[1]
		}
		if len(fields) > 2 {
			entry.Restart = fields[2]
		}
		entries = append(entries, entry)
	}

	return entries
}
