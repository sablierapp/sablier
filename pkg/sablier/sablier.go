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

	groupsMu sync.RWMutex
	groups   map[string][]string
	// groupDeps holds the intra-group dependency graph: group → instance → deps.
	// Populated by SetGroupsFromConfigurations during reconciliation.
	groupDeps map[string]map[string][]string

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

	// rejectUnlabeledRequests blocks direct named requests unless sablier.enable=true.
	rejectUnlabeledRequests bool

	// verifyEnabledOnExpiration re-checks sablier.enable before stopping expired instances.
	verifyEnabledOnExpiration bool

	metrics metrics.Recorder

	l *slog.Logger
}

func New(logger *slog.Logger, store Store, provider Provider) *Sablier {
	return &Sablier{
		provider:                      provider,
		sessions:                      store,
		groupsMu:                      sync.RWMutex{},
		groups:                        map[string][]string{},
		groupDeps:                     map[string]map[string][]string{},
		pendingStarts:                 map[string]*pendingStart{},
		l:                             logger,
		metrics:                       metrics.Noop{},
		BlockingRefreshFrequency:      5 * time.Second,
		InstanceStartTimeout:          5 * time.Minute,
		ExternallyStartedScanInterval: 30 * time.Second,
		RunningHoursRefreshFrequency:  30 * time.Second,
	}
}

// WithRejectUnlabeledRequests makes direct named requests require sablier.enable=true.
func (s *Sablier) WithRejectUnlabeledRequests(reject bool) {
	s.rejectUnlabeledRequests = reject
}

// WithVerifyEnabledOnExpiration makes Sablier re-check sablier.enable before stopping expired instances.
func (s *Sablier) WithVerifyEnabledOnExpiration(verify bool) {
	s.verifyEnabledOnExpiration = verify
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
	}
}

// SetGroupsFromConfigurations sets group membership and dependency information
// from the provider's InstanceGroups result. Group names and their ordered
// member lists are stored in s.groups; dependency graphs (instance → deps) are
// stored in s.groupDeps and used for wave-based startup ordering.
func (s *Sablier) SetGroupsFromConfigurations(configurations map[string][]InstanceConfiguration) {
	names := make(map[string][]string, len(configurations))
	deps := make(map[string]map[string][]string, len(configurations))

	for group, instances := range configurations {
		groupNames := make([]string, 0, len(instances))
		for _, inst := range instances {
			groupNames = append(groupNames, inst.Name)
		}
		names[group] = groupNames

		// Only store a dep map for this group if at least one instance has deps.
		var groupDeps map[string][]string
		for _, inst := range instances {
			if len(inst.DependsOn) > 0 {
				if groupDeps == nil {
					groupDeps = make(map[string][]string)
				}
				groupDeps[inst.Name] = inst.DependsOn
			}
		}
		if groupDeps != nil {
			deps[group] = groupDeps
		}
	}

	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	if diff := cmp.Diff(s.groups, names); diff != "" {
		s.l.Info("set groups", slog.Any("old", s.groups), slog.Any("new", names), slog.Any("diff", diff))
		s.groups = names
	}
	s.groupDeps = deps
}

// GroupDeps returns a defensive copy of the dependency graph for the given group.
// Returns nil if the group has no dependency information.
func (s *Sablier) GroupDeps(group string) map[string][]string {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	src, ok := s.groupDeps[group]
	if !ok {
		return nil
	}
	out := make(map[string][]string, len(src))
	for k, v := range src {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// GroupForInstance returns the group the instance currently belongs to, or empty string.
func (s *Sablier) GroupForInstance(name string) string {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	return s.groupForInstanceLocked(name)
}

// groupForInstanceLocked scans s.groups to find which group instance belongs to.
// Must be called with groupsMu held.
func (s *Sablier) groupForInstanceLocked(name string) string {
	for group, instances := range s.groups {
		for _, inst := range instances {
			if inst == name {
				return group
			}
		}
	}
	return ""
}

// AddInstanceToGroup adds instance to the given group, removing it from any previous group.
// Returns the previous group (empty if none) so the caller can log the transition.
func (s *Sablier) AddInstanceToGroup(instance, group string) (previous string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	previous = s.groupForInstanceLocked(instance)
	if previous == group {
		return previous
	}
	if previous != "" {
		s.removeFromGroup(instance, previous)
	}
	s.groups[group] = append(s.groups[group], instance)
	return previous
}

// RemoveInstanceFromGroup removes instance from whichever group it belongs to.
// Returns the group it was removed from (empty if it wasn't in any group).
func (s *Sablier) RemoveInstanceFromGroup(instance string) (group string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	group = s.groupForInstanceLocked(instance)
	if group == "" {
		return ""
	}
	s.removeFromGroup(instance, group)
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
