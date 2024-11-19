package provider

import "time"

type StartOptions struct {
	DesiredReplicas    uint32
	ConsiderReadyAfter time.Duration
}

type ListOptions struct {
	// All list all instances whatever their status (up or down)
	All bool
}
