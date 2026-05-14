package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
)

// parseCPUNano converts a decimal CPU value (e.g. "0.5", "2") to nanocores.
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
// to bytes using Docker-style suffixes.
func parseMemoryBytes(memory string) (int64, error) {
	b, err := units.RAMInBytes(memory)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value %q: %w", memory, err)
	}
	return b, nil
}

// ServiceUpdateScale updates the replica count and/or CPU/memory resource limits
// of a Swarm service in a single service update call. Pass an empty string for
// cpu or memory to set that limit to 0 (unlimited).
func (p *Provider) ServiceUpdateScale(ctx context.Context, name string, replicas uint64, cpu, memory string) error {
	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return fmt.Errorf("cannot get service: %w", err)
	}

	if service.Spec.Mode.Replicated == nil {
		return errors.New("swarm service is not in \"replicated\" mode")
	}

	service.Spec.Mode.Replicated.Replicas = &replicas

	if cpu != "" || memory != "" {
		if service.Spec.TaskTemplate.Resources == nil {
			service.Spec.TaskTemplate.Resources = &swarm.ResourceRequirements{}
		}
		if service.Spec.TaskTemplate.Resources.Limits == nil {
			service.Spec.TaskTemplate.Resources.Limits = &swarm.Limit{}
		}
		if cpu != "" {
			v, err := parseCPUNano(cpu)
			if err != nil {
				return err
			}
			service.Spec.TaskTemplate.Resources.Limits.NanoCPUs = v
		}
		if memory != "" {
			v, err := parseMemoryBytes(memory)
			if err != nil {
				return err
			}
			service.Spec.TaskTemplate.Resources.Limits.MemoryBytes = v
		}
	}

	response, err := p.Client.ServiceUpdate(ctx, service.ID, client.ServiceUpdateOptions{
		Version: service.Version,
		Spec:    service.Spec,
	})
	if err != nil {
		return fmt.Errorf("cannot update service resources: %w", err)
	}

	if len(response.Warnings) > 0 {
		return fmt.Errorf("warning received updating swarm service [%s]: %s", service.Spec.Name, strings.Join(response.Warnings, ", "))
	}

	return nil
}
