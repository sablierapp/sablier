package awsecs_test

import (
	"errors"
	"testing"

	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider/awsecs"
)

func TestNew(t *testing.T) {
	t.Parallel()

	f := newFake()
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)
	assert.Assert(t, p != nil)
	assert.Equal(t, f.callCount("DescribeClusters"), 1)
}

func TestNew_ClusterNotFound(t *testing.T) {
	t.Parallel()

	_, err := awsecs.New(t.Context(), newFake(), "missing", slogt.New(t))
	assert.ErrorContains(t, err, `ECS cluster "missing" not found`)
	assert.ErrorContains(t, err, "MISSING")
}

func TestNew_ClusterInactive(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.clusterStatus = "INACTIVE"
	_, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.ErrorContains(t, err, "is INACTIVE, expected ACTIVE")
}

func TestNew_APIError(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.describeErr = errors.New("boom")
	_, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.ErrorContains(t, err, "cannot connect to the ECS API: boom")
}
