package sablier

import (
	"context"
	"log/slog"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
)

// GroupWatch maintains group membership in sync with the provider.
//
// It subscribes to created, updated, and removed instance events to react
// immediately to changes, and runs a reconciliation loop every 30 seconds
// as a safety net in case events are missed.
//
// Per-instance log messages are emitted for every group assignment change:
//   - "instance X added to group A (reason: created)"
//   - "instance X moved from group A to group B (reason: updated)"
//   - "instance X removed from group A (reason: removed)"
func (s *Sablier) GroupWatch(ctx context.Context) {
	stream := s.provider.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{
			provider.InstanceEventCreated,
			provider.InstanceEventUpdated,
			provider.InstanceEventRemoved,
		},
	})
	eventsC := stream.Events
	errC := stream.Err

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial reconciliation so current state matches provider labels.
	s.reconcileGroups(ctx, "startup")

	for {
		select {
		case <-ctx.Done():
			s.l.InfoContext(ctx, "stop watching groups", slog.Any("reason", ctx.Err()))
			return

		case event, ok := <-eventsC:
			if !ok {
				s.l.WarnContext(ctx, "group event stream closed; relying on reconciliation ticker")
				eventsC = nil // disable this case
				continue
			}
			s.handleGroupEvent(ctx, event)

		case err, ok := <-errC:
			if !ok {
				errC = nil
				continue
			}
			s.l.ErrorContext(ctx, "group event stream permanently lost; relying on reconciliation ticker", slog.Any("error", err))
			errC = nil
			eventsC = nil

		case <-ticker.C:
			s.reconcileGroups(ctx, "reconciliation")
			// Once anti-affinity is in use, keep its index fresh (a safety net for
			// missed events) and re-enforce so a missed suppress/restore self-heals.
			if s.hasAntiAffinity() {
				s.reconcileAntiAffinityRegistry(ctx)
				s.reconcileAntiAffinity(ctx)
			}
		}
	}
}

// handleGroupEvent processes a single instance lifecycle event and updates group membership.
func (s *Sablier) handleGroupEvent(ctx context.Context, event InstanceEvent) {
	info := event.Info
	reason := string(event.Type)

	// Keep the anti-affinity index in step with the instance's current labels,
	// independently of group membership (an anti-affinity instance need not
	// belong to a group). Re-enforce afterwards so a newly declared or removed
	// anti-affinity takes effect immediately.
	s.handleAntiAffinityEvent(ctx, event)

	switch event.Type {
	case provider.InstanceEventCreated:
		if len(info.Groups) == 0 {
			return // not group-managed
		}
		added, _ := s.SyncInstanceGroups(info.Name, info.Groups)
		for _, g := range added {
			s.l.InfoContext(ctx, "instance added to group",
				slog.String("instance", info.Name),
				slog.String("group", g),
				slog.String("reason", reason),
			)
		}

	case provider.InstanceEventUpdated:
		currentGroups := s.GroupsForInstance(info.Name)
		newGroups := info.Groups
		if equalStringSliceSets(currentGroups, newGroups) {
			return // groups unchanged
		}
		if len(newGroups) == 0 {
			// Instance lost all group labels.
			removed := s.RemoveInstanceFromAllGroups(info.Name)
			for _, g := range removed {
				s.l.InfoContext(ctx, "instance removed from group",
					slog.String("instance", info.Name),
					slog.String("group", g),
					slog.String("reason", reason),
				)
			}
			return
		}
		added, removed := s.SyncInstanceGroups(info.Name, newGroups)
		for _, g := range added {
			s.l.InfoContext(ctx, "instance added to group",
				slog.String("instance", info.Name),
				slog.String("group", g),
				slog.String("reason", reason),
			)
		}
		for _, g := range removed {
			s.l.InfoContext(ctx, "instance removed from group",
				slog.String("instance", info.Name),
				slog.String("group", g),
				slog.String("reason", reason),
			)
		}

	case provider.InstanceEventRemoved:
		removed := s.RemoveInstanceFromAllGroups(info.Name)
		for _, g := range removed {
			s.l.InfoContext(ctx, "instance removed from group",
				slog.String("instance", info.Name),
				slog.String("group", g),
				slog.String("reason", reason),
			)
		}
	}
}

// reconcileGroups performs a full group resync against the provider.
func (s *Sablier) reconcileGroups(ctx context.Context, reason string) {
	groups, err := s.provider.InstanceGroups(ctx)
	if err != nil {
		s.l.ErrorContext(ctx, "cannot retrieve groups from provider", slog.String("reason", reason), slog.Any("error", err))
		return
	}
	if groups == nil {
		return
	}

	// Build a new instanceToGroups from the provider's authoritative state.
	newInstanceToGroups := make(map[string][]string)
	for group, instances := range groups {
		for _, inst := range instances {
			newInstanceToGroups[inst] = append(newInstanceToGroups[inst], group)
		}
	}

	// Sync additions and changes.
	for inst, newGroups := range newInstanceToGroups {
		added, removed := s.SyncInstanceGroups(inst, newGroups)
		for _, g := range added {
			s.l.InfoContext(ctx, "instance added to group",
				slog.String("instance", inst),
				slog.String("group", g),
				slog.String("reason", reason),
			)
		}
		for _, g := range removed {
			s.l.InfoContext(ctx, "instance removed from group",
				slog.String("instance", inst),
				slog.String("group", g),
				slog.String("reason", reason),
			)
		}
	}

	// Detect complete removals (instances no longer in any group).
	for inst := range s.groups.InstanceSnapshot() {
		if _, stillManaged := newInstanceToGroups[inst]; !stillManaged {
			removed := s.RemoveInstanceFromAllGroups(inst)
			for _, g := range removed {
				s.l.InfoContext(ctx, "instance removed from group",
					slog.String("instance", inst),
					slog.String("group", g),
					slog.String("reason", reason),
				)
			}
		}
	}
}

// equalStringSliceSets returns true if a and b contain the same elements (order-independent).
func equalStringSliceSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	setA := stringSet(a)
	for _, v := range b {
		if !setA[v] {
			return false
		}
	}
	return true
}
