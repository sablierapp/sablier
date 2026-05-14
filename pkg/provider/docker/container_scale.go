package docker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// parseCPUNano converts a decimal CPU value (e.g. "0.5", "2") to Docker nanocores.
// 1 CPU = 1,000,000,000 nanocores.
func parseCPUNano(cpu string) (int64, error) {
	v, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value %q: %w", cpu, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("CPU value must be non-negative, got %q", cpu)
	}
	return int64(v * 1e9), nil
}

// parseMemoryBytes converts a human-readable memory string (e.g. "128m", "1g")
// to bytes using Docker-style suffixes (b, k, m, g).
func parseMemoryBytes(memory string) (int64, error) {
	b, err := units.RAMInBytes(memory)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value %q: %w", memory, err)
	}
	return b, nil
}

// applyResources updates the CPU and/or memory limits of a running container
// using cgroup constraints (docker update). Pass an empty string for cpu or
// memory to leave that limit unchanged (a zero value removes the limit).
func (p *Provider) applyResources(ctx context.Context, name, cpu, memory string) error {
	resources := &container.Resources{}

	if cpu != "" {
		v, err := parseCPUNano(cpu)
		if err != nil {
			return err
		}
		resources.NanoCPUs = v
	}

	if memory != "" {
		v, err := parseMemoryBytes(memory)
		if err != nil {
			return err
		}
		resources.Memory = v
		// Docker requires MemorySwap >= Memory in the same update call.
		// Setting MemorySwap equal to Memory satisfies the constraint and
		// disables swap for the container.
		resources.MemorySwap = v
	}

	result, err := p.Client.ContainerUpdate(ctx, name, client.ContainerUpdateOptions{
		Resources: resources,
	})
	if err != nil {
		return fmt.Errorf("cannot update resources for container %s: %w", name, err)
	}
	if len(result.Warnings) > 0 {
		p.l.WarnContext(ctx, "warnings from container resource update", "name", name, "warnings", result.Warnings)
	}
	return nil
}
