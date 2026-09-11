package nomad_test

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestInstanceStop(t *testing.T) {
	t.Run("scales a running task group to zero", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 2, enabledMeta))
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "whoami/web")

		assert.NilError(t, err)
		calls := m.getScaleCalls()
		assert.Equal(t, len(calls), 1)
		assert.Equal(t, calls[0].JobID, "whoami")
		assert.Equal(t, calls[0].Group, "web")
		assert.Equal(t, calls[0].Count, int64(0))
		assert.Assert(t, calls[0].Message != "", "scaling message is empty")
	})

	t.Run("does nothing when the task group is already at zero", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 0, enabledMeta))
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, len(m.getScaleCalls()), 0)
	})

	t.Run("honors the idle replicas label", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 3, map[string]string{"sablier.enable": "true", "sablier.idle.replicas": "1"}))
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "whoami/web")

		assert.NilError(t, err)
		calls := m.getScaleCalls()
		assert.Equal(t, len(calls), 1)
		assert.Equal(t, calls[0].Count, int64(1))
	})

	t.Run("registers the job with the idle count when a deployment blocks scaling", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 1, enabledMeta))
		m.setActiveDeployment("whoami", true)
		index := m.jobModifyIndex("whoami")
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, len(m.getScaleCalls()), 0)
		registered := m.getRegistered()
		assert.Equal(t, len(registered), 1)
		assert.Equal(t, *registered[0].Job.TaskGroups[0].Count, 0)
		assert.Equal(t, registered[0].EnforceIndex, true)
		assert.Equal(t, registered[0].JobModifyIndex, index)
	})

	t.Run("returns other scaling errors without registering the job", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("whoami", 1, enabledMeta))
		m.setScaleFail(true)
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "whoami/web")

		assert.ErrorContains(t, err, `cannot scale task group "web" of job "whoami" to 0`)
		assert.Equal(t, len(m.getRegistered()), 0)
	})

	t.Run("does nothing when the job is stopped", func(t *testing.T) {
		m := newMockNomad(t)
		job := serviceJob("whoami", 1, enabledMeta)
		job.Stop = new(true)
		m.addJob(job)
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "whoami/web")

		assert.NilError(t, err)
		assert.Equal(t, len(m.getScaleCalls()), 0)
	})

	t.Run("rejects an unknown job", func(t *testing.T) {
		m := newMockNomad(t)
		p := m.provider(t)

		err := p.InstanceStop(t.Context(), "missing/web")

		assert.ErrorContains(t, err, `job "missing" not found`)
	})
}
