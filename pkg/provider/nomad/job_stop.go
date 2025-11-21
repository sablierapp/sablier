package nomad

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/api"
)

// InstanceStop scales the Nomad job's task group to zero
func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "stopping instance", "name", name)

	jobID, groupName, err := parseJobName(name)
	if err != nil {
		return err
	}

	jobs := p.Client.Jobs()
	job, _, err := jobs.Info(jobID, &api.QueryOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return fmt.Errorf("cannot get job info: %w", err)
	}

	if job == nil {
		return fmt.Errorf("job %s not found", jobID)
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
		return fmt.Errorf("task group %s not found in job %s", groupName, jobID)
	}

	// Check if already at zero
	currentCount := int32(0)
	if targetGroup.Count != nil {
		currentCount = int32(*targetGroup.Count)
	}

	if currentCount == 0 {
		p.l.DebugContext(ctx, "instance already stopped",
			"name", name,
		)
		return nil
	}

	// Scale to zero
	count := 0
	targetGroup.Count = &count

	// Submit the job update
	_, _, err = jobs.Register(job, &api.WriteOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return fmt.Errorf("cannot stop job: %w", err)
	}

	p.l.InfoContext(ctx, "scaled instance down",
		"name", name,
		"from", currentCount,
		"to", 0,
	)

	return nil
}
