package awsecs_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider/awsecs"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func TestInstanceStop(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 1, 1, enabledTags(nil)))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStop(t.Context(), "web"))

	updates := f.recordedUpdates()
	assert.Equal(t, len(updates), 1)
	assert.Equal(t, aws.ToInt32(updates[0].DesiredCount), int32(0))

	got, err := p.InstanceInspect(t.Context(), "web")
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStopped)
}

func TestInstanceStop_IdleReplicas(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 2, 2, enabledTags(map[string]string{sablier.LabelIdleReplicas: "1"})))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStop(t.Context(), "web"))

	updates := f.recordedUpdates()
	assert.Equal(t, len(updates), 1)
	assert.Equal(t, aws.ToInt32(updates[0].DesiredCount), int32(1))
}

func TestInstanceStop_AlreadyStopped(t *testing.T) {
	t.Parallel()

	f := newFake(newService("web", 0, 0, enabledTags(nil)))
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStop(t.Context(), "web"))
	assert.Equal(t, f.callCount("UpdateService"), 0)
}

func TestInstanceStop_NotFound(t *testing.T) {
	t.Parallel()

	p, err := awsecs.New(t.Context(), newFake(), testCluster, slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStop(t.Context(), "missing")
	assert.ErrorContains(t, err, "not found")
}
