package nomad

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// InstanceInspect retrieves the current state of a Nomad job's task group
func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	jobID, groupName, err := parseJobName(name)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	jobs := p.Client.Jobs()
	job, _, err := jobs.Info(jobID, &api.QueryOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot get job info: %w", err)
	}

	if job == nil {
		return sablier.InstanceInfo{}, fmt.Errorf("job %s not found", jobID)
	}

	// Find the task group
	var targetGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if tg.Name != nil && *tg.Name == groupName {
			targetGroup = tg
			break
		}
	}

	if targetGroup == nil {
		return sablier.InstanceInfo{}, fmt.Errorf("task group %s not found in job %s", groupName, jobID)
	}

	desiredCount := int32(0)
	if targetGroup.Count != nil {
		desiredCount = int32(*targetGroup.Count)
	}

	// Get allocations for this job to determine actual running count
	allocations, _, err := jobs.Allocations(jobID, false, &api.QueryOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot get job allocations: %w", err)
	}

	// Count running allocations for this specific task group
	runningCount := int32(0)
	for _, alloc := range allocations {
		if alloc.TaskGroup == groupName && (alloc.ClientStatus == "running" || alloc.ClientStatus == "pending") {
			runningCount++
		}
	}

	instanceName := formatJobName(jobID, groupName)

	// Determine status
	if desiredCount == 0 {
		return sablier.NotReadyInstanceState(instanceName, runningCount, p.desiredReplicas), nil
	}

	// Check if all allocations are running
	if runningCount == desiredCount && desiredCount > 0 {
		// Check allocation health for task groups with health checks
		allHealthy := true
		for _, alloc := range allocations {
			if alloc.TaskGroup == groupName && alloc.ClientStatus == "running" {
				// If DeploymentStatus exists and Health is set, check it
				if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil {
					if !*alloc.DeploymentStatus.Healthy {
						allHealthy = false
						break
					}
				}
			}
		}

		if allHealthy {
			return sablier.ReadyInstanceState(instanceName, desiredCount), nil
		}
	}

	return sablier.NotReadyInstanceState(instanceName, runningCount, desiredCount), nil
}
