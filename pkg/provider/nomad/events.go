package nomad

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/api"
)

// NotifyInstanceStopped watches for job allocations being stopped/completed
// and sends the instance name to the channel when detected
func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	p.l.InfoContext(ctx, "starting nomad events watcher")

	// Use Nomad's event stream API to watch for allocation updates
	topics := map[api.Topic][]string{
		api.TopicAllocation: {"*"},
	}

	streamCh, err := p.Client.EventStream().Stream(ctx, topics, 0, &api.QueryOptions{
		Namespace: p.namespace,
	})

	if err != nil {
		p.l.ErrorContext(ctx, "failed to start event stream", "error", err)
		return
	}

	p.l.InfoContext(ctx, "nomad event stream started")

	// Track last seen count for each task group to detect scale-downs
	lastSeen := make(map[string]int32)

	// Poll job allocations periodically as a fallback
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.l.InfoContext(ctx, "stopping nomad events watcher")
			return

		case event := <-streamCh:
			if event.Err != nil {
				p.l.ErrorContext(ctx, "event stream error", "error", event.Err)
				continue
			}

			// Process allocation events
			for _, e := range event.Events {
				if e.Type == "AllocationUpdated" {
					p.processAllocationEvent(ctx, e, instance, lastSeen)
				}
			}

		case <-ticker.C:
			// Periodically check all jobs for scale-down as a fallback
			p.pollJobAllocations(ctx, instance, lastSeen)
		}
	}
}

func (p *Provider) processAllocationEvent(ctx context.Context, event api.Event, instance chan<- string, lastSeen map[string]int32) {
	alloc, ok := event.Payload["Allocation"]
	if !ok {
		return
	}

	allocMap, ok := alloc.(map[string]interface{})
	if !ok {
		return
	}

	jobID, _ := allocMap["JobID"].(string)
	taskGroup, _ := allocMap["TaskGroup"].(string)
	clientStatus, _ := allocMap["ClientStatus"].(string)

	if jobID == "" || taskGroup == "" {
		return
	}

	// If allocation stopped, check if this was a scale-down
	if clientStatus == "complete" || clientStatus == "failed" || clientStatus == "lost" {
		instanceName := formatJobName(jobID, taskGroup)

		// Check current job state
		info, err := p.InstanceInspect(ctx, instanceName)
		if err != nil {
			p.l.WarnContext(ctx, "cannot inspect instance after allocation event",
				"instance", instanceName,
				"error", err,
			)
			return
		}

		// If scaled to zero, notify
		if info.DesiredReplicas == 0 {
			p.l.InfoContext(ctx, "instance scaled to zero detected",
				"instance", instanceName,
			)
			select {
			case instance <- instanceName:
			case <-ctx.Done():
			}
		}
	}
}

func (p *Provider) pollJobAllocations(ctx context.Context, instance chan<- string, lastSeen map[string]int32) {
	jobs := p.Client.Jobs()
	jobList, _, err := jobs.List(&api.QueryOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		p.l.WarnContext(ctx, "failed to list jobs for polling", "error", err)
		return
	}

	for _, jobStub := range jobList {
		job, _, err := jobs.Info(jobStub.ID, &api.QueryOptions{
			Namespace: p.namespace,
		})
		if err != nil {
			continue
		}

		for _, tg := range job.TaskGroups {
			if tg.Name == nil || tg.Meta == nil {
				continue
			}

			// Only watch enabled instances
			enabled, hasEnable := tg.Meta[enableLabel]
			if !hasEnable || enabled != "true" {
				continue
			}

			instanceName := formatJobName(*job.ID, *tg.Name)
			currentCount := int32(0)
			if tg.Count != nil {
				currentCount = int32(*tg.Count)
			}

			// Check if scaled down to zero
			if prev, exists := lastSeen[instanceName]; exists && prev > 0 && currentCount == 0 {
				p.l.InfoContext(ctx, "instance scaled to zero detected via polling",
					"instance", instanceName,
					"previous_count", prev,
				)
				select {
				case instance <- instanceName:
				case <-ctx.Done():
					return
				}
			}

			lastSeen[instanceName] = currentCount
		}
	}
}
