package sablier

import (
	"fmt"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestStartupStats_TypicalIsTheMedianOfRecentStartups(t *testing.T) {
	st := newStartupStats()
	now := time.Unix(1_000_000, 0)

	assert.Equal(t, st.typical("web"), time.Duration(0))

	st.observe("web", 30*time.Second, now)
	assert.Equal(t, st.typical("web"), 30*time.Second)

	st.observe("web", 50*time.Second, now)
	assert.Equal(t, st.typical("web"), 40*time.Second)

	// A startup measured long after the fact (nobody checked the instance
	// while it started) does not move the median much.
	st.observe("web", 40*time.Second, now)
	st.observe("web", 2*time.Hour, now)
	st.observe("web", 35*time.Second, now)
	assert.Equal(t, st.typical("web"), 40*time.Second)

	// Zero and negative durations are not startups.
	st.observe("web", 0, now)
	st.observe("web", -time.Second, now)
	assert.Equal(t, st.typical("web"), 40*time.Second)
}

func TestStartupStats_KeepsTheLastStartupsOnly(t *testing.T) {
	st := newStartupStats()
	now := time.Unix(1_000_000, 0)

	for range startupSamples {
		st.observe("web", 10*time.Minute, now)
	}
	// The instance got faster: once the window is full of fast startups,
	// the slow ones are forgotten.
	for range startupSamples {
		st.observe("web", 20*time.Second, now)
	}
	assert.Equal(t, st.typical("web"), 20*time.Second)
}

func TestStartupStats_ForgetsTheInstanceObservedLeastRecently(t *testing.T) {
	st := newStartupStats()
	start := time.Unix(1_000_000, 0)

	for i := range maxStartupStatsInstances {
		st.observe(fmt.Sprintf("ci-%d", i), time.Minute, start.Add(time.Duration(i)*time.Second))
	}
	// ci-0 was observed first, then observed again: ci-1 is now the oldest.
	st.observe("ci-0", time.Minute, start.Add(time.Hour))
	st.observe("new", 2*time.Minute, start.Add(2*time.Hour))

	assert.Equal(t, len(st.entries), maxStartupStatsInstances)
	assert.Equal(t, st.typical("ci-1"), time.Duration(0))
	assert.Equal(t, st.typical("ci-0"), time.Minute)
	assert.Equal(t, st.typical("new"), 2*time.Minute)
}

func TestStartupStats_NilIsEmpty(t *testing.T) {
	var st *startupStats
	st.observe("web", time.Minute, time.Now())
	assert.Equal(t, st.typical("web"), time.Duration(0))
}
