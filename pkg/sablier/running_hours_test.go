package sablier_test

import (
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestParseRunningHours(t *testing.T) {
	tests := []struct {
		name  string
		value string
		err   bool
	}{
		{name: "valid day window", value: "08:30-17:45", err: false},
		{name: "valid overnight window", value: "22:00-06:00", err: false},
		{name: "invalid format", value: "08:30/17:45", err: true},
		{name: "invalid hour", value: "25:00-26:00", err: true},
		{name: "same start and end", value: "10:00-10:00", err: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sablier.ParseRunningHours(tt.value)
			if tt.err {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestRunningHoursWindowAt(t *testing.T) {
	dayWindow, err := sablier.ParseRunningHours("08:00-12:00")
	assert.NilError(t, err)

	loc := time.Local
	_, dayEnd, in := dayWindow.WindowAt(time.Date(2025, 1, 5, 9, 30, 0, 0, loc))
	assert.Assert(t, in)
	assert.Equal(t, dayEnd.Hour(), 12)
	assert.Equal(t, dayEnd.Minute(), 0)

	_, _, in = dayWindow.WindowAt(time.Date(2025, 1, 5, 7, 59, 0, 0, loc))
	assert.Assert(t, !in)

	overnight, err := sablier.ParseRunningHours("22:00-06:00")
	assert.NilError(t, err)

	_, overnightEnd, in := overnight.WindowAt(time.Date(2025, 1, 5, 23, 30, 0, 0, loc))
	assert.Assert(t, in)
	assert.Equal(t, overnightEnd.Day(), 6)
	assert.Equal(t, overnightEnd.Hour(), 6)

	_, overnightEnd, in = overnight.WindowAt(time.Date(2025, 1, 6, 1, 30, 0, 0, loc))
	assert.Assert(t, in)
	assert.Equal(t, overnightEnd.Day(), 6)
	assert.Equal(t, overnightEnd.Hour(), 6)
}

func TestParseRunningDays(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []time.Weekday
		err   bool
	}{
		{name: "abbreviations", value: "Mon,Tue,Wed,Thu,Fri", want: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}},
		{name: "full names", value: "Saturday, Sunday", want: []time.Weekday{time.Saturday, time.Sunday}},
		{name: "case insensitive", value: "mon,SUN", want: []time.Weekday{time.Monday, time.Sunday}},
		{name: "unknown day", value: "Mon,Funday", err: true},
		{name: "empty", value: "", err: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, err := sablier.ParseRunningDays(tt.value)
			if tt.err {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			for _, d := range tt.want {
				assert.Assert(t, days.Contains(d), "expected %s to be included", d)
			}
			// Days not in want must not be present.
			for d := time.Sunday; d <= time.Saturday; d++ {
				expected := false
				for _, w := range tt.want {
					if w == d {
						expected = true
						break
					}
				}
				assert.Equal(t, days.Contains(d), expected)
			}
		})
	}
}

