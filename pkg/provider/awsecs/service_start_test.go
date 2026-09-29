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

func TestInstanceStart(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 0, 0, enabledTags(nil)))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStart(t.Context(), "web"))

	updates := f.recordedUpdates()
	assert.Equal(t, len(updates), 1)
	assert.Equal(t, aws.ToString(updates[0].Cluster), testCluster)
	assert.Equal(t, aws.ToString(updates[0].Service), serviceArn("web"))
	assert.Equal(t, aws.ToInt32(updates[0].DesiredCount), int32(1))

	got, err := p.InstanceInspect(t.Context(), "web")
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStarting)
}

func TestInstanceStart_ActiveReplicas(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 0, 0, enabledTags(map[string]string{
		sablier.LabelActiveReplicas: "3",
		sablier.LabelActiveCPU:      "2",
	})))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStart(t.Context(), "web"))

	updates := f.recordedUpdates()
	assert.Equal(t, len(updates), 1)
	assert.Equal(t, aws.ToInt32(updates[0].DesiredCount), int32(3))
}

func TestInstanceStart_AlreadyRunning(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStart(t.Context(), "web"))
	assert.Equal(t, f.callCount("UpdateService"), 0)
}

func TestInstanceStart_NotFound(t *testing.T) {
	t.Parallel()

	p, err := awsecs.New(t.Context(), newFake(), testCluster, slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), "missing")
	assert.ErrorContains(t, err, "not found")
}

func TestInstanceStart_Daemon(t *testing.T) {
	t.Parallel()

	svc := newService("agent", 0, 2, enabledTags(nil))
	svc.SchedulingStrategy = types.SchedulingStrategyDaemon
	f := newFake(svc)
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), "agent")
	assert.ErrorContains(t, err, "DAEMON scheduling strategy")
	assert.Equal(t, f.callCount("UpdateService"), 0)
}

func TestInstanceStart_UpdateError(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 0, 0, enabledTags(nil)))
	f.updateErr = errors.New("AccessDeniedException")
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), "web")
	assert.ErrorContains(t, err, `cannot update service "web" desired count: AccessDeniedException`)
}

func TestInstanceDependencies(t *testing.T) {
	t.Parallel()

	p, err := awsecs.New(t.Context(), newFake(), testCluster, slogt.New(t))
	assert.NilError(t, err)

	deps, err := p.InstanceDependencies(t.Context(), "web")
	assert.NilError(t, err)
	assert.Assert(t, deps == nil)
}
