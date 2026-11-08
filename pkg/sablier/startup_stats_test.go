package sablier

import (
	"fmt"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// observeWakes records one startup of d per wake, each with its own start time.
func observeWakes(st *startupStats, name string, now time.Time, durations ...time.Duration) {
	for i, d := range durations {
		st.observe(name, now.Add(time.Duration(i)*time.Hour), d, now)
	}
}

func TestStartupStats_TypicalIsTheMedianOfRecentStartups(t *testing.T) {
	st := newStartupStats()
	now := time.Unix(1_000_000, 0)

	assert.Equal(t, st.typical("web"), time.Duration(0))

	observeWakes(st, "web", now, 30*time.Second, 50*time.Second, 40*time.Second)
	assert.Equal(t, st.typical("web"), 40*time.Second)

	// Once enough startups are known, one measured long after the fact
	// (nobody checked the instance while it started) does not move the
	// median much.
	st.observe("web", now.Add(10*time.Hour), 2*time.Hour, now)
	st.observe("web", now.Add(11*time.Hour), 35*time.Second, now)
	assert.Equal(t, st.typical("web"), 40*time.Second)

	// Zero and negative durations are not startups.
	st.observe("web", now.Add(12*time.Hour), 0, now)
	st.observe("web", now.Add(13*time.Hour), -time.Second, now)
	assert.Equal(t, st.typical("web"), 40*time.Second)
}

// One late sample used to set the value outright, and two were averaged (40 s
// and 59m40s gave about 30 min): no value is given before three startups.
func TestStartupStats_NoTypicalBeforeThreeStartups(t *testing.T) {
	st := newStartupStats()
	now := time.Unix(1_000_000, 0)

	st.observe("web", now, 59*time.Minute+40*time.Second, now)
	assert.Equal(t, st.typical("web"), time.Duration(0))

	st.observe("web", now.Add(time.Hour), 40*time.Second, now)
	assert.Equal(t, st.typical("web"), time.Duration(0))

	st.observe("web", now.Add(2*time.Hour), 40*time.Second, now)
	assert.Equal(t, st.typical("web"), 40*time.Second)
}

// Several requests can see the same start complete, each with its own
// measure: only the first is a sample. Three 40 s wakes then a 10 min wake
// seen by five overlapping requests used to give 10 min.
func TestStartupStats_OneSamplePerStart(t *testing.T) {
	st := newStartupStats()
	now := time.Unix(1_000_000, 0)

	observeWakes(st, "web", now, 40*time.Second, 40*time.Second, 40*time.Second)

	slowStart := now.Add(24 * time.Hour)
	for i := range 5 {
		st.observe("web", slowStart, 10*time.Minute+time.Duration(i)*time.Second, now)
	}
	assert.Equal(t, len(st.entries["web"].samples), 4)
	assert.Equal(t, st.typical("web"), 40*time.Second)
}

func TestStartupStats_KeepsTheLastStartupsOnly(t *testing.T) {
	st := newStartupStats()
	now := time.Unix(1_000_000, 0)

	for i := range startupSamples {
		st.observe("web", now.Add(time.Duration(i)*time.Hour), 10*time.Minute, now)
	}
	// The instance got faster: once the window is full of fast startups,
	// the slow ones are forgotten.
	for i := range startupSamples {
		st.observe("web", now.Add(time.Duration(startupSamples+i)*time.Hour), 20*time.Second, now)
	}
	assert.Equal(t, st.typical("web"), 20*time.Second)
}

func TestStartupStats_ForgetsTheInstanceObservedLeastRecently(t *testing.T) {
	st := newStartupStats()
	start := time.Unix(1_000_000, 0)

	for i := range maxStartupStatsInstances {
		at := start.Add(time.Duration(i) * time.Second)
		observeWakes(st, fmt.Sprintf("ci-%d", i), at, time.Minute, time.Minute, time.Minute)
	}
	// ci-0 was observed first, then observed again: ci-1 is now the oldest.
	st.observe("ci-0", start.Add(10*time.Hour), time.Minute, start.Add(time.Hour))
	observeWakes(st, "new", start.Add(2*time.Hour), 2*time.Minute, 2*time.Minute, 2*time.Minute)

	assert.Equal(t, len(st.entries), maxStartupStatsInstances)
	assert.Equal(t, st.typical("ci-1"), time.Duration(0))
	assert.Equal(t, st.typical("ci-0"), time.Minute)
	assert.Equal(t, st.typical("new"), 2*time.Minute)
}

func TestStartupStats_NilIsEmpty(t *testing.T) {
	var st *startupStats
	st.observe("web", time.Now(), time.Minute, time.Now())
	assert.Equal(t, st.typical("web"), time.Duration(0))
}
