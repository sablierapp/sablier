package sablier

import (
	"maps"
	"slices"
	"sync"
)

// groupRegistry is a concurrent-safe registry that maps group names to the
// instance names that belong to them.
type groupRegistry struct {
	mu   sync.RWMutex
	data map[string][]string
}

func newGroupRegistry() *groupRegistry {
	return &groupRegistry{data: make(map[string][]string)}
}

// Snapshot returns a deep copy of the entire group→instances map.
func (r *groupRegistry) Snapshot() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string][]string, len(r.data))
	for k, v := range r.data {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// Set replaces the entire registry with groups. Returns the previous snapshot
// and whether the content actually changed; the caller is responsible for
// any logging. A nil map is treated as empty.
func (r *groupRegistry) Set(groups map[string][]string) (old map[string][]string, changed bool) {
	if groups == nil {
		groups = make(map[string][]string)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if maps.EqualFunc(r.data, groups, slices.Equal) {
		return nil, false
	}

	old = make(map[string][]string, len(r.data))
	for k, v := range r.data {
		cp := make([]string, len(v))
		copy(cp, v)
		old[k] = cp
	}
	r.data = groups
	return old, true
}

// Get returns the instance list for a group and whether the group exists.
func (r *groupRegistry) Get(group string) ([]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names, ok := r.data[group]
	return names, ok
}

// Keys returns a snapshot of all current group names.
func (r *groupRegistry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Collect(maps.Keys(r.data))
}

// GroupsOf returns all groups the named instance currently belongs to.
func (r *groupRegistry) GroupsOf(instance string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.groupsOf(instance)
}

// Sync sets the instance's group memberships to exactly newGroups, adding and
// removing entries as needed. Returns the groups added to and removed from.
func (r *groupRegistry) Sync(instance string, newGroups []string) (added, removed []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current := stringSet(r.groupsOf(instance))
	desired := stringSet(newGroups)

	for g := range desired {
		if !current[g] {
			r.data[g] = append(r.data[g], instance)
			added = append(added, g)
		}
	}
	for g := range current {
		if !desired[g] {
			r.removeFromGroup(instance, g)
			removed = append(removed, g)
		}
	}
	return added, removed
}

// Remove drops the instance from every group it belongs to and returns those groups.
func (r *groupRegistry) Remove(instance string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	groups := r.groupsOf(instance)
	for _, g := range groups {
		r.removeFromGroup(instance, g)
	}
	return groups
}

// InstanceSnapshot returns a snapshot of the inverse mapping: instance→groups.
func (r *groupRegistry) InstanceSnapshot() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string][]string)
	for group, instances := range r.data {
		for _, inst := range instances {
			out[inst] = append(out[inst], group)
		}
	}
	return out
}

// groupsOf scans the data for all groups the instance belongs to.
// Must be called with mu held (either read or write).
func (r *groupRegistry) groupsOf(instance string) []string {
	var groups []string
	for group, instances := range r.data {
		for _, inst := range instances {
			if inst == instance {
				groups = append(groups, group)
				break
			}
		}
	}
	return groups
}

// removeFromGroup removes instance from the named group's member slice.
// Must be called with mu held (write).
func (r *groupRegistry) removeFromGroup(instance, group string) {
	members := r.data[group]
	filtered := members[:0]
	for _, m := range members {
		if m != instance {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		delete(r.data, group)
	} else {
		r.data[group] = filtered
	}
}

// stringSet converts a string slice into a set map for O(1) lookup.
func stringSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
