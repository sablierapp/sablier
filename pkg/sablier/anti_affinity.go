package sablier

import (
	"context"
	"errors"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/store"
)

// Anti-affinity lets an instance declare, via the sablier.anti-affinity
// label, one or more groups it must back off from. Whenever any listed group has
// an active session, Sablier forces the declaring instance to its idle state:
// for a plain instance that means stopping it, and for a scale-mode instance
// (sablier.idle.replicas >= 1) it means applying the idle resource profile.
//
// The relationship is one-directional (the declaring instance yields to the
// group, not the other way round) and reactive: reconciliation runs whenever a
// session is created (a group may have become active) or expires (a group may
// have become inactive), plus periodically from GroupWatch as a safety net.
//
// Only instances Sablier actively suppressed — tracked in the suppressed set —
// are restarted when their antagonists become inactive, so an instance that was
// already idle before an antagonist woke up is never spuriously started.
//
// The request path is aware of this too: while an antagonist is active, a request
// for a backing-off instance is reported as not-ready with an explanation (see
// antiAffinityHold) rather than started and immediately suppressed again.

// SyncInstanceAntiAffinity sets the instance's anti-affinity antagonist groups to
// exactly groups, updating the reverse index. Returns the antagonists added and
// removed. A nil/empty groups slice clears the instance's anti-affinity.
func (s *Sablier) SyncInstanceAntiAffinity(instance string, groups []string) (added, removed []string) {
	return s.antiAffinity.Sync(instance, groups)
}

// RemoveInstanceAntiAffinity drops the instance from the anti-affinity reverse
// index entirely. Returns the antagonist groups it was registered against.
func (s *Sablier) RemoveInstanceAntiAffinity(instance string) []string {
	return s.antiAffinity.Remove(instance)
}

// hasAntiAffinity reports whether any instance currently declares an
// anti-affinity. Used to skip reconciliation work when the feature is unused.
func (s *Sablier) hasAntiAffinity() bool {
	return len(s.antiAffinity.Keys()) > 0
}

// SeedAntiAffinity builds the anti-affinity index from the provider's current
// instances and enforces it once. Call it at startup, after the initial group
// scan, so pre-existing instances (which never emit a "created" event) are
// registered, and so persisted sessions or already-running workloads that
// conflict are reconciled immediately instead of only on the first GroupWatch
// tick or session event. Runtime changes are then tracked incrementally by
// GroupWatch.
func (s *Sablier) SeedAntiAffinity(ctx context.Context) {
	s.reconcileAntiAffinityRegistry(ctx)
	s.reconcileAntiAffinity(ctx)
}

// reconcileAntiAffinityRegistry rebuilds the anti-affinity reverse index by
// listing every Sablier-enabled instance and reading its
// sablier.anti-affinity label. It is the authoritative source used at
// startup (to pick up pre-existing instances that never emit a "created" event)
// and, once at least one anti-affinity exists, as a periodic safety net against
// missed events. GroupWatch keeps the index fresh incrementally in between.
func (s *Sablier) reconcileAntiAffinityRegistry(ctx context.Context) {
	instances, err := s.provider.InstanceList(ctx, provider.InstanceListOptions{All: true})
	if err != nil {
		s.l.ErrorContext(ctx, "anti-affinity: cannot list instances to rebuild registry", slog.Any("error", err))
		return
	}

	present := make(map[string]bool, len(instances))
	for _, inst := range instances {
		present[inst.Name] = true
		info, inspectErr := s.provider.InstanceInspect(ctx, inst.Name)
		if inspectErr != nil {
			s.l.WarnContext(ctx, "anti-affinity: cannot inspect instance while rebuilding registry",
				slog.String("instance", inst.Name), slog.Any("error", inspectErr))
			continue
		}
		s.SyncInstanceAntiAffinity(inst.Name, info.AntiAffinity)
	}

	// Drop instances that are no longer present.
	for inst := range s.antiAffinity.InstanceSnapshot() {
		if !present[inst] {
			s.RemoveInstanceAntiAffinity(inst)
		}
	}
}

// triggerAntiAffinityReconcile runs a reconciliation in the background when at
// least one anti-affinity is declared. The reconcile is detached from ctx's
// cancellation (so it survives the triggering request completing) but keeps its
// values, e.g. the OTel trace context.
func (s *Sablier) triggerAntiAffinityReconcile(ctx context.Context) {
	if !s.hasAntiAffinity() {
		return
	}
	go s.reconcileAntiAffinity(context.WithoutCancel(ctx))
}

// handleAntiAffinityEvent updates the anti-affinity index from a single instance
// lifecycle event and, if that changed anything, re-enforces so a newly declared
// or removed anti-affinity takes effect immediately. Created/Updated events carry
// the instance's current labels in Info.AntiAffinity; Removed drops it entirely.
func (s *Sablier) handleAntiAffinityEvent(ctx context.Context, event InstanceEvent) {
	name := event.Info.Name
	if name == "" {
		return
	}

	var added, removed []string
	switch event.Type {
	case provider.InstanceEventCreated, provider.InstanceEventUpdated:
		added, removed = s.SyncInstanceAntiAffinity(name, event.Info.AntiAffinity)
	case provider.InstanceEventRemoved:
		removed = s.RemoveInstanceAntiAffinity(name)
		// A removed instance is gone; if it was suppressed, forget it so we do
		// not try to restore an instance that no longer exists.
		s.affinityMu.Lock()
		delete(s.suppressed, name)
		s.affinityMu.Unlock()
	default:
		return
	}

	if len(added) > 0 || len(removed) > 0 {
		s.reconcileAntiAffinity(ctx)
	}
}

// reconcileAntiAffinity brings every anti-affinity instance in line with the
// current session state: instances whose antagonist group is active are forced
// idle, and instances Sablier previously suppressed are restored once none of
// their antagonists remain active.
//
// It is serialised by affinityMu so overlapping activations and expirations
// cannot interleave and leave the suppressed set inconsistent.
func (s *Sablier) reconcileAntiAffinity(ctx context.Context) {
	s.affinityMu.Lock()
	defer s.affinityMu.Unlock()

	// antagonist group -> instances that back off from it.
	registry := s.antiAffinity.Snapshot()
	if len(registry) == 0 {
		return
	}

	// Evaluate each antagonist group once.
	active := make(map[string]bool, len(registry))
	for group := range registry {
		active[group] = s.isGroupActive(ctx, group)
	}

	// An instance must be suppressed if ANY of its antagonist groups is active.
	desiredSuppress := make(map[string]bool)
	for group, dependents := range registry {
		for _, d := range dependents {
			if active[group] {
				desiredSuppress[d] = true
			} else if _, seen := desiredSuppress[d]; !seen {
				desiredSuppress[d] = false
			}
		}
	}

	for instance, suppress := range desiredSuppress {
		_, alreadySuppressed := s.suppressed[instance]
		switch {
		case suppress && !alreadySuppressed:
			s.suppressForAntiAffinity(ctx, instance)
		case !suppress && alreadySuppressed:
			s.restoreFromAntiAffinity(ctx, instance)
		}
	}
}

// suppressForAntiAffinity forces instance to its idle state, but only if it is
// currently active (has a live session). An instance that is already idle is
// left untouched and is not recorded as suppressed, so it will not be started
// later when its antagonist becomes inactive.
//
// Must be called with affinityMu held.
func (s *Sablier) suppressForAntiAffinity(ctx context.Context, instance string) {
	if _, err := s.sessions.Get(ctx, instance); errors.Is(err, store.ErrKeyNotFound) {
		// No active session: the instance is already idle/stopped, nothing to do.
		return
	} else if err != nil {
		s.l.WarnContext(ctx, "anti-affinity: cannot read session, skipping suppression",
			slog.String("instance", instance), slog.Any("error", err))
		return
	}

	s.l.InfoContext(ctx, "anti-affinity: forcing instance idle", slog.String("instance", instance))
	if err := s.provider.InstanceStop(ctx, instance); err != nil {
		s.l.ErrorContext(ctx, "anti-affinity: cannot force instance idle",
			slog.String("instance", instance), slog.Any("error", err))
		return
	}
	// Drop the session so the instance no longer counts as active and does not
	// get stopped a second time by the normal expiration path.
	if err := s.sessions.Delete(ctx, instance); err != nil {
		s.l.WarnContext(ctx, "anti-affinity: cannot delete session after suppression",
			slog.String("instance", instance), slog.Any("error", err))
	}
	s.suppressed[instance] = struct{}{}
	s.metrics.RecordInstanceStop(instance, "anti-affinity")
}

// restoreFromAntiAffinity brings a previously-suppressed instance back once none
// of its antagonist groups are active anymore, by requesting a fresh session for
// it exactly as an ordinary request would. Going through the request path
// re-establishes the session (with its TTL), marks it as started by Sablier — so
// WatchAndStopExternallyStarted does not mistake it for an externally-started
// instance and stop it again — resolves its depends_on, and, in scale mode,
// re-applies the active resource profile. A failed request leaves the instance
// marked suppressed so a later reconcile retries it.
//
// Must be called with affinityMu held.
func (s *Sablier) restoreFromAntiAffinity(ctx context.Context, instance string) {
	s.l.InfoContext(ctx, "anti-affinity: restoring instance", slog.String("instance", instance))
	if _, err := s.instanceRequest(ctx, instance, s.DefaultSessionDuration, false); err != nil {
		s.l.ErrorContext(ctx, "anti-affinity: cannot restore instance",
			slog.String("instance", instance), slog.Any("error", err))
		return
	}
	delete(s.suppressed, instance)
}

// antiAffinityHold reports the first active antagonist group that currently
// forces name to stay idle, or "" when name is free to start. It lets the
// request path avoid starting an instance the background reconcile would
// immediately suppress, and surface the reason to the caller instead.
func (s *Sablier) antiAffinityHold(ctx context.Context, name string) string {
	for _, group := range s.antiAffinity.GroupsOf(name) {
		if s.isGroupActive(ctx, group) {
			return group
		}
	}
	return ""
}

// isGroupActive reports whether any member of group currently holds a live
// session. A group with no members, or one that is unknown, is not active.
func (s *Sablier) isGroupActive(ctx context.Context, group string) bool {
	groups := s.groups.Snapshot()
	members, ok := groups[group]
	if !ok {
		return false
	}
	for _, member := range members {
		_, err := s.sessions.Get(ctx, member)
		if err == nil {
			return true
		}
		if !errors.Is(err, store.ErrKeyNotFound) {
			s.l.WarnContext(ctx, "anti-affinity: cannot read session while checking group activity",
				slog.String("group", group), slog.String("instance", member), slog.Any("error", err))
		}
	}
	return false
}
