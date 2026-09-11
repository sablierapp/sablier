package nomad

import (
	"log/slog"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// testJob builds a service job "whoami" with a single task group "web".
func testJob(count int, meta map[string]string) (*api.Job, *api.TaskGroup) {
	tg := &api.TaskGroup{
		Name:  new("web"),
		Count: new(count),
		Meta:  meta,
	}
	job := &api.Job{
		ID:         new("whoami"),
		Name:       new("whoami"),
		Namespace:  new("default"),
		Type:       new(api.JobTypeService),
		Stop:       new(false),
		TaskGroups: []*api.TaskGroup{tg},
	}
	return job, tg
}

func alloc(group, clientStatus, desiredStatus string) *api.AllocationListStub {
	return &api.AllocationListStub{
		ID:            "alloc-" + clientStatus,
		JobID:         "whoami",
		TaskGroup:     group,
		ClientStatus:  clientStatus,
		DesiredStatus: desiredStatus,
	}
}

func healthy(a *api.AllocationListStub, h *bool) *api.AllocationListStub {
	a.DeploymentStatus = &api.AllocDeploymentStatus{Healthy: h}
	return a
}

func failedAlloc(message string) *api.AllocationListStub {
	a := alloc("web", api.AllocClientStatusFailed, api.AllocDesiredStatusRun)
	a.TaskStates = map[string]*api.TaskState{
		"server": {
			State:  "dead",
			Failed: true,
			Events: []*api.TaskEvent{
				{Type: api.TaskStarted, DisplayMessage: "Task started by client"},
				{Type: api.TaskTerminated, DisplayMessage: message},
			},
		},
	}
	return a
}

func TestInfoFromGroup(t *testing.T) {
	enabled := map[string]string{"sablier.enable": "true"}
	p := &Provider{namespace: "default", l: slog.Default()}

	tests := []struct {
		name   string
		count  int
		meta   map[string]string
		stop   bool
		batch  bool
		allocs []*api.AllocationListStub
		want   sablier.InstanceInfo
	}{
		{
			name:  "count zero is stopped",
			count: 0,
			meta:  enabled,
			want:  sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped},
		},
		{
			name:  "count zero reports the active replicas label as desired",
			count: 0,
			meta:  map[string]string{"sablier.enable": "true", "sablier.active.replicas": "3"},
			want:  sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 3, Status: sablier.InstanceStatusStopped},
		},
		{
			name:   "stopped job is stopped even with a running allocation",
			count:  1,
			meta:   enabled,
			stop:   true,
			allocs: []*api.AllocationListStub{alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped, Message: "job is stopped"},
		},
		{
			name:  "no allocation placed yet is starting",
			count: 1,
			meta:  enabled,
			want:  sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting},
		},
		{
			name:   "pending allocation is starting",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{alloc("web", api.AllocClientStatusPending, api.AllocDesiredStatusRun)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting},
		},
		{
			name:   "running allocation without deployment tracking is ready",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady},
		},
		{
			name:   "running allocation with pending health is starting",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{healthy(alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun), nil)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting, Message: "allocation is running but not healthy yet"},
		},
		{
			name:   "running unhealthy allocation is starting",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{healthy(alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun), new(false))},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting, Message: "allocation is running but not healthy yet"},
		},
		{
			name:   "running healthy allocation is ready",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{healthy(alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun), new(true))},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady},
		},
		{
			name:  "partially ready multi replica group is starting",
			count: 2,
			meta:  enabled,
			allocs: []*api.AllocationListStub{
				healthy(alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun), new(true)),
				alloc("web", api.AllocClientStatusPending, api.AllocDesiredStatusRun),
			},
			want: sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 1, DesiredReplicas: 2, Status: sablier.InstanceStatusStarting},
		},
		{
			name:   "failed allocation without replacement is an error",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{failedAlloc("Exit Code: 1")},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusError, Message: `allocation failed: task "server": Exit Code: 1`},
		},
		{
			name:  "failed allocation with a pending replacement is starting",
			count: 1,
			meta:  enabled,
			allocs: []*api.AllocationListStub{
				failedAlloc("Exit Code: 1"),
				alloc("web", api.AllocClientStatusPending, api.AllocDesiredStatusRun),
			},
			want: sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting},
		},
		{
			name:   "completed batch allocation is completed",
			count:  1,
			meta:   enabled,
			batch:  true,
			allocs: []*api.AllocationListStub{alloc("web", api.AllocClientStatusComplete, api.AllocDesiredStatusRun)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusCompleted},
		},
		{
			name:   "allocation being stopped is ignored",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{alloc("web", api.AllocClientStatusRunning, api.AllocDesiredStatusStop)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting},
		},
		{
			name:   "allocations of other task groups are ignored",
			count:  1,
			meta:   enabled,
			allocs: []*api.AllocationListStub{alloc("db", api.AllocClientStatusRunning, api.AllocDesiredStatusRun)},
			want:   sablier.InstanceInfo{Name: "whoami/web", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, tg := testJob(tt.count, tt.meta)
			job.Stop = new(tt.stop)
			if tt.batch {
				job.Type = new(api.JobTypeBatch)
			}

			got := p.infoFromGroup(job, tg, tt.allocs)

			assert.Equal(t, got.Name, tt.want.Name)
			assert.Equal(t, got.Status, tt.want.Status)
			assert.Equal(t, got.Message, tt.want.Message)
			assert.Equal(t, got.CurrentReplicas, tt.want.CurrentReplicas)
			assert.Equal(t, got.DesiredReplicas, tt.want.DesiredReplicas)
			assert.Equal(t, got.Provider, sablier.ProviderNomad)
			assert.Equal(t, got.Enabled, "true")
			assert.DeepEqual(t, got.Groups, []string{"default"})
			assert.DeepEqual(t, got.Nomad, &sablier.NomadTaskGroupInfo{
				Namespace: "default",
				JobID:     "whoami",
				TaskGroup: "web",
				Meta:      tt.meta,
			})
		})
	}
}

func TestInfoFromGroup_LabelsFromJobAndGroup(t *testing.T) {
	p := &Provider{namespace: "default", l: slog.Default()}
	job, tg := testJob(0, map[string]string{"sablier.group": "team-a,team-b"})
	job.Meta = map[string]string{"sablier.enable": "true", "sablier.group": "job-level", "sablier.ready-after": "30s"}

	got := p.infoFromGroup(job, tg, nil)

	assert.Equal(t, got.Enabled, "true")
	assert.DeepEqual(t, got.Groups, []string{"team-a", "team-b"})
	assert.Equal(t, got.ReadyAfter.String(), "30s")
}
