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
