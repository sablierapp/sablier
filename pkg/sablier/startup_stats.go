package sablier

import (
	"slices"
	"sync"
	"time"
)

// startupSamples is how many of an instance's most recent startups are kept to
// learn its typical startup duration. An odd count gives a true median.
const startupSamples = 7

// minStartupSamples is how many startups must be observed before a typical
// duration is given. With fewer, one startup measured late (the first request
// after the start came long after the instance was ready) would set the value
// outright, or be averaged with a single real one.
const minStartupSamples = 3

// maxStartupStatsInstances bounds how many instances startupStats remembers, so
// that short-lived instances (one per CI pipeline, for example) cannot grow it
// forever. The instance observed least recently is forgotten first.
const maxStartupStatsInstances = 2048

// startupStats learns how long each instance takes to become ready after
// Sablier starts it: from the dispatch of the start to the first request that
// sees the instance ready, plus the instance's ready-after delay. The waiting
// page uses it to tell how long a start usually takes. It is kept in memory
// only, and is learned again after a restart.
type startupStats struct {
	mu      sync.Mutex
	entries map[string]*startupEntry
}

type startupEntry struct {
	samples []time.Duration // most recent last, at most startupSamples
	// lastStart is the start time of the last startup recorded, so that the
	// requests that see the same start complete add one sample, not one each.
	lastStart time.Time
	seen      time.Time
}

func newStartupStats() *startupStats {
	return &startupStats{entries: map[string]*startupEntry{}}
}

// observe records that name, started at startedAt, became ready d after its
// start. A start already recorded for name is ignored: several requests can see
// the same start complete.
func (st *startupStats) observe(name string, startedAt time.Time, d time.Duration, now time.Time) {
	if st == nil || d <= 0 {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	e, ok := st.entries[name]
	if ok && e.lastStart.Equal(startedAt) {
		return
	}
	if !ok {
		if len(st.entries) >= maxStartupStatsInstances {
			st.evictOldestLocked()
		}
		e = &startupEntry{}
		st.entries[name] = e
	}
	e.samples = append(e.samples, d)
	if len(e.samples) > startupSamples {
		e.samples = e.samples[len(e.samples)-startupSamples:]
	}
	e.lastStart = startedAt
	e.seen = now
}

func (st *startupStats) evictOldestLocked() {
	var oldest string
	var oldestSeen time.Time
	for name, e := range st.entries {
		if oldest == "" || e.seen.Before(oldestSeen) {
			oldest, oldestSeen = name, e.seen
		}
	}
	delete(st.entries, oldest)
}

// typical returns the median of the recent startup durations of name, or zero
// until minStartupSamples startups have been observed. The median ignores a
// startup measured late once enough real ones are known, not before.
func (st *startupStats) typical(name string) time.Duration {
	if st == nil {
		return 0
	}
	st.mu.Lock()
	e, ok := st.entries[name]
	var samples []time.Duration
	if ok {
		samples = slices.Clone(e.samples)
	}
	st.mu.Unlock()

	n := len(samples)
	if n < minStartupSamples {
		return 0
	}
	slices.Sort(samples)
	if n%2 == 1 {
		return samples[n/2]
	}
	return (samples[n/2-1] + samples[n/2]) / 2
}
