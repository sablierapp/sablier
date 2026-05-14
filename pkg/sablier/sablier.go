package sablier

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sablierapp/sablier/pkg/metrics"
)

type Sablier struct {
	provider Provider
	sessions Store

	groupsMu        sync.RWMutex
	groups          map[string][]string
	instanceToGroup map[string]string // reverse map: instance → group

	pendingMu     sync.Mutex
	pendingStarts map[string]*pendingStart

	// BlockingRefreshFrequency is the frequency at which the instances are checked
	// against the provider. Defaults to 5 seconds.
	BlockingRefreshFrequency time.Duration

	// InstanceStartTimeout is the maximum time allowed for an async InstanceStart
	// call before it is cancelled. Defaults to 5 minutes.
	InstanceStartTimeout time.Duration

	// ExternallyStartedScanInterval is how often WatchAndStopExternallyStarted performs a
	// full reconciliation scan. Defaults to 30 seconds.
	ExternallyStartedScanInterval time.Duration

	// RunningHoursRefreshFrequency is how often running-hours windows are
	// reconciled. Defaults to 30 seconds.
	RunningHoursRefreshFrequency time.Duration

	metrics metrics.Recorder

	l *slog.Logger
}

func New(logger *slog.Logger, store Store, provider Provider) *Sablier {
	return &Sablier{
		provider:                      provider,
		sessions:                      store,
		groupsMu:                      sync.RWMutex{},
		groups:                        map[string][]string{},
		instanceToGroup:               map[string]string{},
		pendingStarts:                 map[string]*pendingStart{},
		l:                             logger,
		metrics:                       metrics.Noop{},
		BlockingRefreshFrequency:      5 * time.Second,
		InstanceStartTimeout:          5 * time.Minute,
		ExternallyStartedScanInterval: 30 * time.Second,
		RunningHoursRefreshFrequency:  30 * time.Second,
	}
}

// WithMetrics installs a Recorder. Defaults to metrics.Noop until called.
func (s *Sablier) WithMetrics(r metrics.Recorder) {
	if r == nil {
		r = metrics.Noop{}
	}
	s.metrics = r
}

// Groups returns a defensive copy of the current group→instances map. Safe for
// concurrent use; intended for the metrics GroupLockCollector.
func (s *Sablier) Groups() map[string][]string {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	out := make(map[string][]string, len(s.groups))
	for k, v := range s.groups {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func (s *Sablier) SetGroups(groups map[string][]string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	if groups == nil {
		groups = map[string][]string{}
	}
	if diff := cmp.Diff(s.groups, groups); diff != "" {
		s.l.Info("set groups", slog.Any("old", s.groups), slog.Any("new", groups), slog.Any("diff", diff))
		s.groups = groups
		// Rebuild reverse map.
		s.instanceToGroup = make(map[string]string, len(groups))
		for group, instances := range groups {
			for _, inst := range instances {
				s.instanceToGroup[inst] = group
			}
		}
	}
}

// GroupForInstance returns the group the instance currently belongs to, or empty string.
func (s *Sablier) GroupForInstance(name string) string {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	return s.instanceToGroup[name]
}

// AddInstanceToGroup adds instance to the given group, removing it from any previous group.
// Returns the previous group (empty if none) so the caller can log the transition.
func (s *Sablier) AddInstanceToGroup(instance, group string) (previous string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	previous = s.instanceToGroup[instance]
	if previous == group {
		return previous
	}
	// Remove from previous group.
	if previous != "" {
		s.removeFromGroup(instance, previous)
	}
	// Add to new group.
	s.instanceToGroup[instance] = group
	s.groups[group] = append(s.groups[group], instance)
	return previous
}

// RemoveInstanceFromGroup removes instance from whichever group it belongs to.
// Returns the group it was removed from (empty if it wasn't in any group).
func (s *Sablier) RemoveInstanceFromGroup(instance string) (group string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	group = s.instanceToGroup[instance]
	if group == "" {
		return ""
	}
	s.removeFromGroup(instance, group)
	delete(s.instanceToGroup, instance)
	return group
}

// removeFromGroup removes instance from the groups slice. Must be called with groupsMu held.
func (s *Sablier) removeFromGroup(instance, group string) {
	members := s.groups[group]
	filtered := members[:0]
	for _, m := range members {
		if m != instance {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		delete(s.groups, group)
	} else {
		s.groups[group] = filtered
	}
}

func (s *Sablier) RemoveInstance(ctx context.Context, name string) error {
	return s.sessions.Delete(ctx, name)
}
