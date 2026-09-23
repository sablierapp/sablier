package nomad_test

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestInstanceStart(t *testing.T) {
	t.Run("scales a task group at zero to one", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 0, enabledMeta))
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "whoami/web")

		assert.NilError(t, err)
		calls := m.getScaleCalls()
		assert.Equal(t, len(calls), 1)
		assert.Equal(t, calls[0].JobID, "whoami")
		assert.Equal(t, calls[0].Group, "web")
		assert.Equal(t, calls[0].Count, int64(1))
		assert.Assert(t, calls[0].Message != "", "scaling message is empty")
	})

	t.Run("does nothing when the task group is already at the desired count", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 1, enabledMeta))
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, len(m.getScaleCalls()), 0)
	})

	t.Run("honors the active replicas label", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 0, map[string]string{"sablier.enable": "true", "sablier.active.replicas": "3"}))
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "whoami/web")

		assert.NilError(t, err)
		calls := m.getScaleCalls()
		assert.Equal(t, len(calls), 1)
		assert.Equal(t, calls[0].Count, int64(3))
	})

	t.Run("registers a stopped job again instead of scaling it", func(t *testing.T) {
		m := newMockNomad(t)
		job := serviceJob("whoami", 0, enabledMeta)
		job.Stop = new(true)
		m.addJob(job)
		index := m.jobModifyIndex("whoami")
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, len(m.getScaleCalls()), 0)
		registered := m.getRegistered()
		assert.Equal(t, len(registered), 1)
		assert.Equal(t, *registered[0].Job.Stop, false)
		assert.Equal(t, *registered[0].Job.TaskGroups[0].Count, 1)
		assert.Equal(t, registered[0].EnforceIndex, true)
		assert.Equal(t, registered[0].JobModifyIndex, index)
	})

	t.Run("registers the job with the active count when a deployment blocks scaling", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 0, map[string]string{"sablier.enable": "true", "sablier.active.replicas": "2"}))
		m.setActiveDeployment("whoami", true)
		index := m.jobModifyIndex("whoami")
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, len(m.getScaleCalls()), 0)
		registered := m.getRegistered()
		assert.Equal(t, len(registered), 1)
		assert.Equal(t, *registered[0].Job.Stop, false)
		assert.Equal(t, *registered[0].Job.TaskGroups[0].Count, 2)
		assert.Equal(t, registered[0].EnforceIndex, true)
		assert.Equal(t, registered[0].JobModifyIndex, index)
	})

	t.Run("rejects an unknown job", func(t *testing.T) {
		m := newMockNomad(t)
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "missing/web")

		assert.ErrorContains(t, err, `job "missing" not found`)
	})

	t.Run("rejects an unknown task group", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 0, enabledMeta))
		p := m.provider(t)

		err := p.InstanceStart(t.Context(), "whoami/db")

		assert.ErrorContains(t, err, `task group "db" not found`)
	})
}
