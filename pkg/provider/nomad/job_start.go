package nomad

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/api"
)

// InstanceStart scales the Nomad job's task group to the desired replica count
func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	p.l.DebugContext(ctx, "starting instance", "name", name)

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

	// Check if already at desired count
	currentCount := int32(0)
	if targetGroup.Count != nil {
		currentCount = int32(*targetGroup.Count)
	}

	if currentCount == p.desiredReplicas {
		p.l.DebugContext(ctx, "instance already at desired replicas",
			"name", name,
			"current", currentCount,
			"desired", p.desiredReplicas,
		)
		return nil
	}

	// Scale up
	count := int(p.desiredReplicas)
	targetGroup.Count = &count

	// Submit the job update
	_, _, err = jobs.Register(job, &api.WriteOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return fmt.Errorf("cannot scale job: %w", err)
	}

	p.l.InfoContext(ctx, "scaled instance up",
		"name", name,
		"from", currentCount,
		"to", p.desiredReplicas,
	)

	return nil
}
