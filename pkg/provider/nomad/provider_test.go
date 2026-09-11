package nomad_test

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestNew(t *testing.T) {
	t.Run("verifies the connection with the configured namespace and token", func(t *testing.T) {
		m := newMockNomad(t)

		_, err := nomad.New(t.Context(), m.client(t), "team-a", slogt.New(t))

		assert.NilError(t, err)
		assert.Assert(t, m.getNamespaces()["team-a"] > 0, "namespace query parameter not sent")
		assert.Assert(t, m.getTokens()["test-token"] > 0, "ACL token header not sent")
	})

	t.Run("defaults to the default namespace", func(t *testing.T) {
		m := newMockNomad(t)

		_, err := nomad.New(t.Context(), m.client(t), "", slogt.New(t))

		assert.NilError(t, err)
		assert.Assert(t, m.getNamespaces()["default"] > 0, "default namespace not sent")
	})

	t.Run("reports an unreachable API", func(t *testing.T) {
		m := newMockNomad(t)
		m.setListFail(true)

		_, err := nomad.New(t.Context(), m.client(t), "default", slogt.New(t))

		assert.ErrorContains(t, err, "cannot connect to Nomad")
	})
}

func TestInstanceList(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("running", 1, enabledMeta))
	m.addJob(serviceJob("scaled-to-zero", 0, map[string]string{"sablier.enable": "true", "sablier.group": "team-a"}))
	stopped := serviceJob("stopped", 1, enabledMeta)
	stopped.Stop = new(true)
	m.addJob(stopped)
	m.addJob(serviceJob("unlabeled", 1, nil))
	periodic := serviceJob("periodic-parent", 1, enabledMeta)
	periodic.Periodic = &api.PeriodicConfig{Spec: new("* * * * *")}
	m.addJob(periodic)
	p := m.provider(t)

	t.Run("all lists every enabled task group", func(t *testing.T) {
		instances, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})

		assert.NilError(t, err)
		assert.DeepEqual(t, instances, []sablier.InstanceConfiguration{
			{Name: "running/web", Groups: []string{"default"}, Enabled: "true"},
			{Name: "scaled-to-zero/web", Groups: []string{"team-a"}, Enabled: "true"},
			{Name: "stopped/web", Groups: []string{"default"}, Enabled: "true"},
		})
	})

	t.Run("without all lists only the running task groups", func(t *testing.T) {
		instances, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: false})

		assert.NilError(t, err)
		assert.DeepEqual(t, instances, []sablier.InstanceConfiguration{
			{Name: "running/web", Groups: []string{"default"}, Enabled: "true"},
		})
	})

	t.Run("reuses the cached job specs while they are unchanged", func(t *testing.T) {
		before := m.getInfoCalls("running")

		_, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
		assert.NilError(t, err)
		_, err = p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
		assert.NilError(t, err)

		assert.Equal(t, m.getInfoCalls("running"), before)

		// A change to the job invalidates its cache entry.
		m.addJob(serviceJob("running", 2, enabledMeta))
		_, err = p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
		assert.NilError(t, err)
		assert.Equal(t, m.getInfoCalls("running"), before+1)
	})
}

func TestInstanceGroups(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("running", 1, enabledMeta))
	m.addJob(serviceJob("shared", 0, map[string]string{"sablier.enable": "true", "sablier.group": "team-b,team-a"}))
	m.addJob(serviceJob("unlabeled", 1, nil))
	multi := serviceJob("multi", 0, map[string]string{"sablier.enable": "true", "sablier.group": "team-a"})
	multi.TaskGroups = append(multi.TaskGroups, &api.TaskGroup{Name: new("db"), Count: new(0), Meta: map[string]string{"sablier.enable": "true", "sablier.group": "team-a"}})
	m.addJob(multi)
	p := m.provider(t)

	groups, err := p.InstanceGroups(t.Context())

	assert.NilError(t, err)
	assert.DeepEqual(t, groups, map[string][]string{
		"default": {"running/web"},
		"team-a":  {"multi/db", "multi/web", "shared/web"},
		"team-b":  {"shared/web"},
	})
}

func TestInstanceInspect(t *testing.T) {
	m := newMockNomad(t)
	m.addJob(serviceJob("whoami", 1, enabledMeta))
	m.setAllocs("whoami", runningAlloc("whoami"))
	p := m.provider(t)

	t.Run("reports the task group state", func(t *testing.T) {
		info, err := p.InstanceInspect(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, info.Name, "whoami/web")
		assert.Equal(t, info.Status, sablier.InstanceStatusReady)
		assert.Equal(t, info.CurrentReplicas, int32(1))
		assert.Equal(t, info.Provider, sablier.ProviderNomad)
		assert.Equal(t, info.Nomad.JobID, "whoami")
	})

	t.Run("rejects an unknown job", func(t *testing.T) {
		_, err := p.InstanceInspect(t.Context(), "missing/web")

		assert.ErrorContains(t, err, `job "missing" not found in namespace "default"`)
	})

	t.Run("rejects an unknown task group", func(t *testing.T) {
		_, err := p.InstanceInspect(t.Context(), "whoami/db")

		assert.ErrorContains(t, err, `task group "db" not found in job "whoami"`)
	})

	t.Run("lists the task groups when only the job is given", func(t *testing.T) {
		_, err := p.InstanceInspect(t.Context(), "whoami")

		assert.ErrorContains(t, err, `instance name "whoami" must be in the form job/group`)
		assert.ErrorContains(t, err, `has task groups [web]`)
	})

	t.Run("uses the configured namespace", func(t *testing.T) {
		_, _ = p.InstanceInspect(t.Context(), "whoami/web")
		assert.Assert(t, m.getNamespaces()["default"] > 0)
	})
}

func TestNewForTest_Backoff(t *testing.T) {
	m := newMockNomad(t)
	_, err := nomad.NewForTest(t.Context(), m.client(t), "default", slogt.New(t), time.Millisecond)
	assert.NilError(t, err)
}
