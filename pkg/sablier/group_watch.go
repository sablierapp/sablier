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
		}
	}
}

// handleGroupEvent processes a single instance lifecycle event and updates group membership.
func (s *Sablier) handleGroupEvent(ctx context.Context, event InstanceEvent) {
	info := event.Info
	reason := string(event.Type)

	switch event.Type {
	case provider.InstanceEventCreated:
		if info.Group == "" {
			return // not group-managed
		}
		previous := s.AddInstanceToGroup(info.Name, info.Group)
		if previous == "" {
			s.l.InfoContext(ctx, "instance added to group",
				slog.String("instance", info.Name),
				slog.String("group", info.Group),
				slog.String("reason", reason),
			)
		} else if previous != info.Group {
			s.l.InfoContext(ctx, "instance moved to group",
				slog.String("instance", info.Name),
				slog.String("from_group", previous),
				slog.String("to_group", info.Group),
				slog.String("reason", reason),
			)
		}

	case provider.InstanceEventUpdated:
		currentGroup := s.GroupForInstance(info.Name)
		newGroup := info.Group
		if currentGroup == newGroup {
			return // group unchanged
		}
		if newGroup == "" {
			// Instance lost its group label.
			removed := s.RemoveInstanceFromGroup(info.Name)
			if removed != "" {
				s.l.InfoContext(ctx, "instance removed from group",
					slog.String("instance", info.Name),
					slog.String("group", removed),
					slog.String("reason", reason),
				)
			}
			return
		}
		previous := s.AddInstanceToGroup(info.Name, newGroup)
		if previous == "" {
			s.l.InfoContext(ctx, "instance added to group",
				slog.String("instance", info.Name),
				slog.String("group", newGroup),
				slog.String("reason", reason),
			)
		} else {
			s.l.InfoContext(ctx, "instance moved to group",
				slog.String("instance", info.Name),
				slog.String("from_group", previous),
				slog.String("to_group", newGroup),
				slog.String("reason", reason),
			)
		}

	case provider.InstanceEventRemoved:
		group := s.RemoveInstanceFromGroup(info.Name)
		if group != "" {
			s.l.InfoContext(ctx, "instance removed from group",
				slog.String("instance", info.Name),
				slog.String("group", group),
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

	// Build a new instanceToGroup from the provider's authoritative state and
	// emit per-instance log messages for any changes.
	newInstanceToGroup := make(map[string]string)
	for group, instances := range groups {
		for _, inst := range instances {
			newInstanceToGroup[inst] = group
		}
	}

	// Detect additions and moves.
	for inst, newGroup := range newInstanceToGroup {
		oldGroup := s.GroupForInstance(inst)
		if oldGroup == newGroup {
			continue
		}
		s.AddInstanceToGroup(inst, newGroup)
		if oldGroup == "" {
			s.l.InfoContext(ctx, "instance added to group",
				slog.String("instance", inst),
				slog.String("group", newGroup),
				slog.String("reason", reason),
			)
		} else {
			s.l.InfoContext(ctx, "instance moved to group",
				slog.String("instance", inst),
				slog.String("from_group", oldGroup),
				slog.String("to_group", newGroup),
				slog.String("reason", reason),
			)
		}
	}

	// Detect removals.
	for inst, oldGroup := range s.instanceToGroupSnapshot() {
		if _, stillManaged := newInstanceToGroup[inst]; !stillManaged {
			s.RemoveInstanceFromGroup(inst)
			s.l.InfoContext(ctx, "instance removed from group",
				slog.String("instance", inst),
				slog.String("group", oldGroup),
				slog.String("reason", reason),
			)
		}
	}
}

// instanceToGroupSnapshot returns a snapshot of the current instance→group mapping,
// derived from s.groups. Safe for concurrent use.
func (s *Sablier) instanceToGroupSnapshot() map[string]string {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	out := make(map[string]string)
	for group, instances := range s.groups {
		for _, inst := range instances {
			out[inst] = group
		}
	}
	return out
}
