package awsecs_test

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider/awsecs"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func enabledTags(extra map[string]string) map[string]string {
	tags := map[string]string{sablier.LabelEnable: "true"}
	for k, v := range extra {
		tags[k] = v
	}
	return tags
}

func TestInstanceInspect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		service     *types.Service
		wantStatus  sablier.InstanceStatus
		wantCurrent int32
		wantDesired int32
		wantMessage string
	}{
		{
			name:        "desired 0 is stopped",
			service:     newService("web", 0, 0, enabledTags(nil)),
			wantStatus:  sablier.InstanceStatusStopped,
			wantCurrent: 0,
			wantDesired: 1,
		},
		{
			name:        "desired 0 with draining tasks is stopped",
			service:     newService("web", 0, 1, enabledTags(nil)),
			wantStatus:  sablier.InstanceStatusStopped,
			wantCurrent: 0,
			wantDesired: 1,
		},
		{
			name:        "running below desired is starting",
			service:     newService("web", 2, 1, enabledTags(nil)),
			wantStatus:  sablier.InstanceStatusStarting,
			wantCurrent: 1,
			wantDesired: 1,
		},
		{
			name:        "rollout in progress is starting",
			service:     withDeployment(newService("web", 1, 1, enabledTags(nil)), types.DeploymentRolloutStateInProgress, ""),
			wantStatus:  sablier.InstanceStatusStarting,
			wantCurrent: 1,
			wantDesired: 1,
		},
		{
			name:        "rollout completed is ready",
			service:     withDeployment(newService("web", 1, 1, enabledTags(nil)), types.DeploymentRolloutStateCompleted, ""),
			wantStatus:  sablier.InstanceStatusReady,
			wantCurrent: 1,
			wantDesired: 1,
		},
		{
			name:        "no deployment info and counts match is ready",
			service:     newService("web", 1, 1, enabledTags(nil)),
			wantStatus:  sablier.InstanceStatusReady,
			wantCurrent: 1,
			wantDesired: 1,
		},
		{
			name:        "rolling deployment with extra running tasks is ready",
			service:     withDeployment(newService("web", 1, 2, enabledTags(nil)), types.DeploymentRolloutStateCompleted, ""),
			wantStatus:  sablier.InstanceStatusReady,
			wantCurrent: 2,
			wantDesired: 1,
		},
		{
			name:        "rollout failed is error",
			service:     withDeployment(newService("web", 1, 0, enabledTags(nil)), types.DeploymentRolloutStateFailed, "ECS deployment circuit breaker: tasks failed to start."),
			wantStatus:  sablier.InstanceStatusError,
			wantCurrent: 0,
			wantDesired: 1,
			wantMessage: "deployment ecs-svc/1 failed: ECS deployment circuit breaker: tasks failed to start.",
		},
		{
			name:        "active replicas tag sets the desired replicas",
			service:     newService("web", 3, 3, enabledTags(map[string]string{sablier.LabelActiveReplicas: "3"})),
			wantStatus:  sablier.InstanceStatusReady,
			wantCurrent: 3,
			wantDesired: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := awsecs.New(t.Context(), newFake(tt.service), testCluster, slogt.New(t))
			assert.NilError(t, err)

			got, err := p.InstanceInspect(t.Context(), "web")
			assert.NilError(t, err)
			assert.Equal(t, got.Name, "web")
			assert.Equal(t, got.Status, tt.wantStatus)
			assert.Equal(t, got.CurrentReplicas, tt.wantCurrent)
			assert.Equal(t, got.DesiredReplicas, tt.wantDesired)
			assert.Equal(t, got.Message, tt.wantMessage)
			assert.Equal(t, got.Provider, sablier.ProviderECS)
			assert.Equal(t, got.Enabled, "true")
		})
	}
}

func TestInstanceInspect_Labels(t *testing.T) {
	t.Parallel()

	svc := newService("web", 1, 1, enabledTags(map[string]string{
		sablier.LabelGroup:        "team-a team-b",
		sablier.LabelReadyAfter:   "30s",
		sablier.LabelAntiAffinity: "streaming",
	}))
	p, err := awsecs.New(t.Context(), newFake(svc), testCluster, slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceInspect(t.Context(), "web")
	assert.NilError(t, err)
	assert.DeepEqual(t, got.Groups, []string{"team-a", "team-b"})
	assert.DeepEqual(t, got.AntiAffinity, []string{"streaming"})
	assert.Equal(t, got.ReadyAfter.String(), "30s")
	assert.Assert(t, got.IsEnabled())
}

func TestInstanceInspect_ByArn(t *testing.T) {
	t.Parallel()

	p, err := awsecs.New(t.Context(), newFake(newService("web", 1, 1, nil)), testCluster, slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceInspect(t.Context(), serviceArn("web"))
	assert.NilError(t, err)
	assert.Equal(t, got.Name, "web")
	assert.Equal(t, got.Status, sablier.InstanceStatusReady)
	assert.Equal(t, got.Enabled, "")
	assert.Assert(t, !got.IsEnabled())
}

func TestInstanceInspect_NotFound(t *testing.T) {
	t.Parallel()

	p, err := awsecs.New(t.Context(), newFake(), testCluster, slogt.New(t))
	assert.NilError(t, err)

	_, err = p.InstanceInspect(t.Context(), "missing")
	assert.ErrorContains(t, err, `service "missing" not found in cluster "test-cluster": missing: MISSING`)
}

func TestInstanceInspect_Inactive(t *testing.T) {
	t.Parallel()

	svc := newService("web", 0, 0, nil)
	svc.Status = aws.String("INACTIVE")
	p, err := awsecs.New(t.Context(), newFake(svc), testCluster, slogt.New(t))
	assert.NilError(t, err)

	_, err = p.InstanceInspect(t.Context(), "web")
	assert.ErrorContains(t, err, "service is inactive")
}

func TestInstanceInspect_Daemon(t *testing.T) {
	t.Parallel()

	svc := newService("agent", 0, 3, enabledTags(nil))
	svc.SchedulingStrategy = types.SchedulingStrategyDaemon
	p, err := awsecs.New(t.Context(), newFake(svc), testCluster, slogt.New(t))
	assert.NilError(t, err)

	_, err = p.InstanceInspect(t.Context(), "agent")
	assert.ErrorContains(t, err, "DAEMON scheduling strategy")
}

func TestInstanceInspect_APIError(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, nil))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	f.describeErr = errors.New("throttled")
	_, err = p.InstanceInspect(t.Context(), "web")
	assert.ErrorContains(t, err, `cannot describe service "web": throttled`)
}
