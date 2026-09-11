package awsecs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/awsecs"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func TestInstanceList_All(t *testing.T) {
	t.Parallel()

	daemon := newService("agent", 0, 2, enabledTags(nil))
	daemon.SchedulingStrategy = types.SchedulingStrategyDaemon
	f := newFake(
		newService("web", 1, 1, enabledTags(map[string]string{sablier.LabelGroup: "team-a team-b"})),
		newService("db", 0, 0, enabledTags(nil)),
		newService("untagged", 1, 1, nil),
		newService("disabled", 1, 1, map[string]string{sablier.LabelEnable: "false"}),
		daemon,
	)
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.DeepEqual(t, got, []sablier.InstanceConfiguration{
		{Name: "db", Groups: []string{"default"}, Enabled: "true"},
		{Name: "web", Groups: []string{"team-a", "team-b"}, Enabled: "true"},
	})
}

func TestInstanceList_RunningOnly(t *testing.T) {
	t.Parallel()

	f := newFake(
		newService("web", 1, 0, enabledTags(nil)),
		newService("db", 0, 1, enabledTags(nil)),
	)
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.DeepEqual(t, got, []sablier.InstanceConfiguration{
		{Name: "web", Groups: []string{"default"}, Enabled: "true"},
	})
}

func TestInstanceList_Pagination(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.pageSize = 10
	for i := range 25 {
		f.set(newService(fmt.Sprintf("svc-%02d", i), 1, 1, enabledTags(nil)))
	}
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.Equal(t, len(got), 25)
	assert.Equal(t, got[0].Name, "svc-00")
	assert.Equal(t, got[24].Name, "svc-24")
	assert.Equal(t, f.callCount("ListServices"), 3)
	assert.Equal(t, f.callCount("DescribeServices"), 3)
}

func TestInstanceList_Errors(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		f := newFake(newService("web", 1, 1, enabledTags(nil)))
		p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
		assert.NilError(t, err)
		f.setListErr(errors.New("throttled"))
		_, err = p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
		assert.ErrorContains(t, err, "cannot list services: throttled")
	})

	t.Run("describe", func(t *testing.T) {
		t.Parallel()
		f := newFake(newService("web", 1, 1, enabledTags(nil)))
		p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
		assert.NilError(t, err)
		f.describeErr = errors.New("throttled")
		_, err = p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
		assert.ErrorContains(t, err, "cannot describe services: throttled")
	})
}

func TestInstanceGroups(t *testing.T) {
	t.Parallel()

	f := newFake(
		newService("web", 1, 1, enabledTags(map[string]string{sablier.LabelGroup: "team-a team-b"})),
		newService("api", 0, 0, enabledTags(map[string]string{sablier.LabelGroup: "team-a"})),
		newService("db", 0, 0, enabledTags(nil)),
		newService("untagged", 1, 1, nil),
	)
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceGroups(t.Context())
	assert.NilError(t, err)
	assert.DeepEqual(t, got, map[string][]string{
		"default": {"db"},
		"team-a":  {"api", "web"},
		"team-b":  {"web"},
	})
}

func TestInstanceGroups_Error(t *testing.T) {
	t.Parallel()

	f := newFake()
	p, err := awsecs.New(t.Context(), f, testCluster, slogt.New(t))
	assert.NilError(t, err)
	f.setListErr(errors.New("throttled"))

	_, err = p.InstanceGroups(t.Context())
	assert.ErrorContains(t, err, "cannot list services: throttled")
}
