package awsecs_test

import (
	"testing"

	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider/awsecs"
	"github.com/sablierapp/sablier/pkg/sablier"
)

var _ sablier.InstanceNameResolver = (*awsecs.Provider)(nil)

// A request may name a service by its ARN, but inspect and the session store
// use the service name. See #1185.
func TestProvider_InstanceName(t *testing.T) {
	p, err := awsecs.New(t.Context(), newFake(), testCluster, slogt.New(t))
	assert.NilError(t, err)

	tests := map[string]string{
		"arn:aws:ecs:us-east-1:123456789012:service/apps/web": "web",
		"arn:aws:ecs:us-east-1:123456789012:service/web":      "web", // old ARN format, without the cluster
		"web": "web",
		"arn:aws:ecs:us-east-1:123456789012:task/apps/0123456789abcdef": "arn:aws:ecs:us-east-1:123456789012:task/apps/0123456789abcdef",
	}
	for name, want := range tests {
		assert.Equal(t, p.InstanceName(t.Context(), name), want, name)
	}
}
