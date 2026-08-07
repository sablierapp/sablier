package sablier

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RunningHours represents a daily window in local time where an instance
// should be kept running.
type RunningHours struct {
	startMinute int
	endMinute   int
}

// ParseRunningHours parses a value in the form "HH:MM-HH:MM".
//
// The window is evaluated in local time (respecting TZ when configured).
// Overnight windows are supported, for example "22:00-06:00".
func ParseRunningHours(v string) (RunningHours, error) {
	parts := strings.Split(v, "-")
	if len(parts) != 2 {
		return RunningHours{}, fmt.Errorf("invalid running-hours %q: expected HH:MM-HH:MM", v)
	}

	start, err := parseClock(parts[0])
	if err != nil {
		return RunningHours{}, fmt.Errorf("invalid running-hours %q: %w", v, err)
	}
	end, err := parseClock(parts[1])
	if err != nil {
		return RunningHours{}, fmt.Errorf("invalid running-hours %q: %w", v, err)
	}
	if start == end {
		return RunningHours{}, fmt.Errorf("invalid running-hours %q: start and end cannot be equal", v)
	}

	return RunningHours{startMinute: start, endMinute: end}, nil
}

func parseClock(s string) (int, error) {
	chunks := strings.Split(strings.TrimSpace(s), ":")
	if len(chunks) != 2 {
		return 0, fmt.Errorf("time %q must use HH:MM", s)
	}
	h, err := strconv.Atoi(chunks[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(chunks[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q is out of range", s)
	}
	return h*60 + m, nil
}

// WindowAt returns whether now is inside the running-hours window and, if so,
// the start and end time of the active window.
func (r RunningHours) WindowAt(now time.Time) (start time.Time, end time.Time, in bool) {
	loc := now.Location()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	nowMinute := now.Hour()*60 + now.Minute()

	if r.startMinute < r.endMinute {
		start = midnight.Add(time.Duration(r.startMinute) * time.Minute)
		end = midnight.Add(time.Duration(r.endMinute) * time.Minute)
		return start, end, !now.Before(start) && now.Before(end)
	}

	// Overnight window, e.g. 22:00-06:00.
	if nowMinute >= r.startMinute {
		start = midnight.Add(time.Duration(r.startMinute) * time.Minute)
		end = midnight.Add(24*time.Hour + time.Duration(r.endMinute)*time.Minute)
		return start, end, !now.Before(start) && now.Before(end)
	}

	start = midnight.Add(-24*time.Hour + time.Duration(r.startMinute)*time.Minute)
	end = midnight.Add(time.Duration(r.endMinute) * time.Minute)
	return start, end, !now.Before(start) && now.Before(end)
}

func runningHoursRemaining(hours, days string, now time.Time) (time.Duration, bool, error) {
	rh, err := ParseRunningHours(hours)
	if err != nil {
		return 0, false, err
	}
	start, end, in := rh.WindowAt(now)
	if !in {
		return 0, false, nil
	}
	if days != "" {
		rd, err := ParseRunningDays(days)
		if err != nil {
			return 0, false, err
		}
		// Evaluate the day against the window's start day so overnight windows
		// (e.g. Fri 22:00-06:00) are attributed to the day they began.
		if !rd.Contains(start.Weekday()) {
			return 0, false, nil
		}
	}
	remaining := max(end.Sub(now), 0)
	return remaining, true, nil
}

// RunningDays is the set of weekdays on which a running-hours window applies.
type RunningDays map[time.Weekday]bool

var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// ParseRunningDays parses a comma-separated list of days into a set.
//
// It accepts full names ("Monday") and common abbreviations ("Mon"). Matching
// is case-insensitive and whitespace around entries is ignored.
func ParseRunningDays(v string) (RunningDays, error) {
	days := make(RunningDays)
	for _, p := range strings.Split(v, ",") {
		token := strings.ToLower(strings.TrimSpace(p))
		if token == "" {
			continue
		}
		wd, ok := weekdayNames[token]
		if !ok {
			return nil, fmt.Errorf("invalid running-days %q: unknown day %q", v, strings.TrimSpace(p))
		}
		days[wd] = true
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("invalid running-days %q: no days specified", v)
	}
	return days, nil
}

// Contains reports whether the given weekday is part of the set.
func (d RunningDays) Contains(day time.Weekday) bool {
	return d[day]
}

