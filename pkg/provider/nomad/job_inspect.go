package nomad

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	job, tg, err := p.getGroup(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot inspect instance %q: %w", name, err)
	}

	// Only the allocations of the current job matter. A job registered again
	// after a purge would otherwise report the allocations of the purged job.
	allocs, _, err := p.Client.Jobs().Allocations(deref(job.ID), false, p.queryOptions(ctx))
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cannot list allocations of job %q: %w", deref(job.ID), err)
	}

	var deployment *api.Deployment
	if awaitsHealth(job, tg, allocs) {
		deployment, _, err = p.Client.Jobs().LatestDeployment(deref(job.ID), p.queryOptions(ctx))
		if err != nil {
			return sablier.InstanceInfo{}, fmt.Errorf("cannot get the latest deployment of job %q: %w", deref(job.ID), err)
		}
	}

	info := p.infoFromGroup(job, tg, allocs, deployment)
	p.l.DebugContext(ctx, "task group inspected",
		slog.String("name", info.Name),
		slog.String("status", string(info.Status)),
		slog.Int("current_replicas", int(info.CurrentReplicas)),
		slog.Int("desired_replicas", int(info.DesiredReplicas)),
	)
	return info, nil
}

// infoFromGroup derives the instance state of one task group from the job
// spec and the allocations of the job.
func (p *Provider) infoFromGroup(job *api.Job, tg *api.TaskGroup, allocs []*api.AllocationListStub, deployment *api.Deployment) sablier.InstanceInfo {
	jobID := deref(job.ID)
	group := deref(tg.Name)
	name := instanceName(jobID, group)
	meta := groupMeta(job, tg)

	count := int32(deref(tg.Count))
	desired := count
	if desired <= 0 {
		// The count a start would scale the group to.
		desired = max(sablier.ScaleConfigFromLabels(meta).Active.Replicas, 1)
	}

	var info sablier.InstanceInfo
	switch {
	case deref(job.Stop):
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusStopped,
			Message:         "job is stopped",
		}
	case count == 0:
		info = sablier.InstanceInfo{
			Name:            name,
			CurrentReplicas: 0,
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusStopped,
		}
	default:
		info = groupStatus(name, count, deref(job.Type), group, allocs, tracksHealth(job, group, deployment))
	}

	namespace := deref(job.Namespace)
	if namespace == "" {
		namespace = p.namespace
	}
	sablier.PopulateEnabledAndGroup(&info, meta)
	info.Provider = sablier.ProviderNomad
	info.Nomad = &sablier.NomadTaskGroupInfo{
		Namespace: namespace,
		JobID:     jobID,
		TaskGroup: group,
		Meta:      meta,
	}
	return info
}

// groupStatus derives the state of a task group with a positive count from
// its allocations.
func groupStatus(name string, count int32, jobType, group string, allocs []*api.AllocationListStub, healthTracked bool) sablier.InstanceInfo {
	var ready, live, completed, failed int32
	var unhealthy, failure string

	for _, a := range allocs {
		if a.TaskGroup != group || a.DesiredStatus != api.AllocDesiredStatusRun {
			continue
		}
		switch a.ClientStatus {
		case api.AllocClientStatusRunning:
			switch {
			case a.DeploymentStatus != nil && deref(a.DeploymentStatus.Healthy):
				ready++
			case a.DeploymentStatus != nil || healthTracked:
				live++
				unhealthy = "allocation is running but not healthy yet"
			default:
				// Without a deployment there is no health tracking, so running is the
				// best readiness signal, as with a Docker container without healthcheck.
				ready++
			}
		case api.AllocClientStatusPending:
			live++
		case api.AllocClientStatusComplete:
			completed++
		case api.AllocClientStatusFailed, api.AllocClientStatusLost, api.AllocClientStatusUnknown:
			failed++
			if failure == "" {
				failure = failureMessage(a)
			}
		}
	}

	switch {
	case ready >= count:
		return sablier.InstanceInfo{Name: name, CurrentReplicas: ready, DesiredReplicas: count, Status: sablier.InstanceStatusReady}
	case live > 0:
		return sablier.InstanceInfo{Name: name, CurrentReplicas: ready, DesiredReplicas: count, Status: sablier.InstanceStatusStarting, Message: unhealthy}
	case jobType == api.JobTypeBatch && completed >= count:
		return sablier.InstanceInfo{Name: name, CurrentReplicas: 0, DesiredReplicas: count, Status: sablier.InstanceStatusCompleted}
	case failed > 0:
		return sablier.InstanceInfo{Name: name, CurrentReplicas: ready, DesiredReplicas: count, Status: sablier.InstanceStatusError, Message: "allocation failed: " + failure}
	default:
		// The scheduler has not placed any allocation yet.
		return sablier.InstanceInfo{Name: name, CurrentReplicas: ready, DesiredReplicas: count, Status: sablier.InstanceStatusStarting}
	}
}

// awaitsHealth reports whether a running allocation of the group has no health data.
// Nomad sets it only when the deployment of the allocation decides the health.
func awaitsHealth(job *api.Job, tg *api.TaskGroup, allocs []*api.AllocationListStub) bool {
	if deref(job.Stop) || deref(tg.Count) == 0 {
		return false
	}
	group := deref(tg.Name)
	return slices.ContainsFunc(allocs, func(a *api.AllocationListStub) bool {
		return a.TaskGroup == group && a.DesiredStatus == api.AllocDesiredStatusRun &&
			a.ClientStatus == api.AllocClientStatusRunning && a.DeploymentStatus == nil
	})
}

// tracksHealth reports whether an active deployment of this job decides the health
// of the task group. The job check is the same as in Job.Scale of Nomad.
func tracksHealth(job *api.Job, group string, d *api.Deployment) bool {
	if d == nil || d.JobCreateIndex != deref(job.CreateIndex) {
		return false
	}
	switch d.Status {
	case api.DeploymentStatusSuccessful, api.DeploymentStatusFailed, api.DeploymentStatusCancelled:
		return false
	}
	_, ok := d.TaskGroups[group]
	return ok
}

// failureMessage describes why an allocation failed from its task events.
func failureMessage(a *api.AllocationListStub) string {
	for _, task := range slices.Sorted(maps.Keys(a.TaskStates)) {
		ts := a.TaskStates[task]
		if !ts.Failed || len(ts.Events) == 0 {
			continue
		}
		last := ts.Events[len(ts.Events)-1]
		msg := last.DisplayMessage
		if msg == "" {
			msg = last.Message
		}
		if msg == "" {
			msg = last.Type
		}
		return fmt.Sprintf("task %q: %s", task, msg)
	}
	if a.ClientDescription != "" {
		return a.ClientDescription
	}
	return "allocation " + a.ClientStatus
}
