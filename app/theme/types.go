package theme

import "time"

// Theme represents an available theme
type Theme struct {
	Name     string
	Embedded bool
}

// Instance holds the current state about an instance
type Instance struct {
	Name            string
	Status          string
	Error           error
	CurrentReplicas int
	DesiredReplicas int
}

// Options holds the customizable input to template
type Options struct {
	DisplayName      string
	ShowDetails      bool
	InstanceStates   []Instance
	SessionDuration  time.Duration
	RefreshFrequency time.Duration
}

// templateOptions holds the internal options used to template
type templateOptions struct {
	DisplayName      string
	InstanceStates   []Instance
	SessionDuration  string
	RefreshFrequency string
	Version          string
}
