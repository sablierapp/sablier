package nomad_test

import (
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"gotest.tools/v3/assert"
)

// twoGroupJob is a job with the task groups "web" and "worker", both managed by Sablier.
func twoGroupJob(id string, count int) *api.Job {
	job := serviceJob(id, count, enabledMeta)
	job.TaskGroups = append(job.TaskGroups, &api.TaskGroup{
		Name:  new("worker"),
		Count: new(count),
		Meta:  enabledMeta,
	})
	return job
}

// runTogether runs every call at the same time and returns their errors.
func runTogether(calls ...func() error) []error {
	errs := make([]error, len(calls))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = call()
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func TestConcurrentScale(t *testing.T) {
	const delay = 100 * time.Millisecond

	t.Run("stops two task groups of the same job at the same time", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(twoGroupJob("demo", 1))
		m.setScaleDelay(delay)
		p := m.provider(t)

		errs := runTogether(
			func() error { return p.InstanceStop(t.Context(), "demo/web") },
			func() error { return p.InstanceStop(t.Context(), "demo/worker") },
		)

		assert.NilError(t, errs[0])
		assert.NilError(t, errs[1])
		assert.Equal(t, m.groupCount("demo", "web"), 0)
		assert.Equal(t, m.groupCount("demo", "worker"), 0)
		assert.Equal(t, m.getMaxScaleInFlight(), 1)
	})

	t.Run("starts two task groups of the same job at the same time", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(twoGroupJob("demo", 0))
		m.setScaleDelay(delay)
		p := m.provider(t)

		errs := runTogether(
			func() error { return p.InstanceStart(t.Context(), "demo/web") },
			func() error { return p.InstanceStart(t.Context(), "demo/worker") },
		)

		assert.NilError(t, errs[0])
		assert.NilError(t, errs[1])
		assert.Equal(t, m.groupCount("demo", "web"), 1)
		assert.Equal(t, m.groupCount("demo", "worker"), 1)
		assert.Equal(t, m.getMaxScaleInFlight(), 1)
	})

	t.Run("stops two task groups at the same time while a deployment blocks scaling", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(twoGroupJob("demo", 1))
		m.setActiveDeployment("demo", true)
		m.setScaleDelay(delay)
		p := m.provider(t)

		errs := runTogether(
			func() error { return p.InstanceStop(t.Context(), "demo/web") },
			func() error { return p.InstanceStop(t.Context(), "demo/worker") },
		)

		assert.NilError(t, errs[0])
		assert.NilError(t, errs[1])
		assert.Equal(t, m.groupCount("demo", "web"), 0)
		assert.Equal(t, m.groupCount("demo", "worker"), 0)
	})

	t.Run("scales one task group as before", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(twoGroupJob("demo", 1))
		m.setScaleDelay(delay)
		p := m.provider(t)

		assert.NilError(t, p.InstanceStop(t.Context(), "demo/web"))

		assert.Equal(t, m.groupCount("demo", "web"), 0)
		assert.Equal(t, m.groupCount("demo", "worker"), 1)
		assert.Equal(t, len(m.getScaleCalls()), 1)
	})

	t.Run("does not make task groups of different jobs wait for each other", func(t *testing.T) {
		m := newMockNomad(t)
		m.addJob(serviceJob("one", 1, enabledMeta))
		m.addJob(serviceJob("two", 1, enabledMeta))
		m.setScaleDelay(delay)
		p := m.provider(t)

		errs := runTogether(
			func() error { return p.InstanceStop(t.Context(), "one/web") },
			func() error { return p.InstanceStop(t.Context(), "two/web") },
		)

		assert.NilError(t, errs[0])
		assert.NilError(t, errs[1])
		assert.Equal(t, m.groupCount("one", "web"), 0)
		assert.Equal(t, m.groupCount("two", "web"), 0)
		assert.Equal(t, m.getMaxScaleInFlight(), 2)
	})
}
